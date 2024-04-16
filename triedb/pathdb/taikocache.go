package pathdb

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

type taikoCache struct {
	config *Config

	diskdb  ethdb.Database
	freezer *rawdb.ResettableFreezer

	tailLayer *tailLayer

	ownerPaths *lru.Cache[common.Hash, *ownerPath]
	taikoMetas *lru.Cache[uint64, *taikoMeta]

	tm   time.Time
	lock sync.RWMutex
}

func newTaikoCache(config *Config, diskdb ethdb.Database, freezer *rawdb.ResettableFreezer) *taikoCache {
	return &taikoCache{
		config:  config,
		diskdb:  diskdb,
		freezer: freezer,

		tailLayer:  newTailLayer(diskdb, config.DirtyCacheSize, config.CleanCacheSize),
		ownerPaths: lru.NewCache[common.Hash, *ownerPath](100),
		taikoMetas: lru.NewCache[uint64, *taikoMeta](10000),

		tm: time.Now(),
	}
}

func (t *taikoCache) recordDiffLayer(lyer *diffLayer) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	var (
		start = time.Now()
		batch = t.diskdb.NewBatch()
	)

	// write nodes to disk
	data, err := encodeNodes(lyer.nodes)
	if err != nil {
		return err
	}
	rawdb.WriteNodeHistoryPrefix(t.diskdb, lyer.id, data)

	tailID := t.tailLayer.getTailID()
	for owner, subset := range lyer.nodes {
		paths, ok := t.ownerPaths.Get(owner)
		if !ok {
			paths = newOwnerPath(owner)
			t.ownerPaths.Add(owner, paths)
		}

		paths.addPath(subset, lyer.id)
		// if tail id is updated then truncate the tail.
		if tailID > 1 {
			paths.truncateTail(tailID)
		}
		if err := paths.savePaths(batch); err != nil {
			return err
		}
	}

	// write data to disk.
	size := batch.ValueSize()
	if err = batch.Write(); err != nil {
		log.Error("Failed to write batch", "err", err)
	}

	// try to truncate the tail layer.
	if err = t.truncateFromTail(); err != nil {
		return err
	}

	// log the record layer
	if time.Now().Sub(t.tm) > time.Second*2 {
		t.tm = time.Now()
		log.Info("record layer", "id", lyer.id, "bytes", common.StorageSize(size), "elapsed", common.PrettyDuration(time.Since(start)))
	}

	return nil
}

func (t *taikoCache) Close() error {
	// Truncate the taiko metas
	return t.truncateFromTail()
}

func (t *taikoCache) Reader(root common.Hash) layer {
	lyer, err := newTaikoLayer(t, root)
	if err != nil {
		log.Error("Failed to recover state", "root", root, "err", err)
		return nil
	}
	return lyer
}

func (t *taikoCache) getTailLayer() *tailLayer {
	return t.tailLayer
}

func (t *taikoCache) getLatestIDByPath(owner common.Hash, path string, startID uint64) (uint64, error) {
	paths, ok := t.ownerPaths.Get(owner)
	if !ok {
		// load paths from disk.
		var err error
		paths, err = loadPaths(t.diskdb, owner)
		if err != nil {
			return 0, err
		}
		t.ownerPaths.Add(owner, paths)
	}

	return paths.getLatestID(path, startID)
}

func (t *taikoCache) loadDiffLayer(id uint64) (*taikoMeta, error) {
	if !t.taikoMetas.Contains(id) {
		var err error
		nodes, err := decodeNodes(rawdb.ReadNodeHistoryPrefix(t.diskdb, id))
		if err != nil {
			return nil, err
		}
		t.taikoMetas.Add(id, &taikoMeta{nodes: nodes})
	}
	node, _ := t.taikoMetas.Get(id)
	return node, nil
}

func (t *taikoCache) truncateFromTail() error {
	ohead, err := t.freezer.Ancients()
	if err != nil {
		return err
	}
	if ohead <= t.config.StateHistory {
		return nil
	}
	ntail := ohead - t.config.StateHistory
	// Load the meta objects in range [otail+1, ntail]
	for otail := t.tailLayer.getTailID(); otail < ntail; otail++ {
		nodes, err := t.loadDiffLayer(otail)
		if err != nil {
			return err
		}
		t.tailLayer.commit(nodes.nodes)
		t.tailLayer.setTailID(otail + 1)
	}

	// Truncate the taiko metas
	if err := t.tailLayer.flush(false); err != nil {
		return err
	}

	return nil
}
