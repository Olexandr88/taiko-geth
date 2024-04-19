package pathdb

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie/testutil"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/stretchr/testify/assert"
)

type taikoTester struct {
	db     ethdb.Database
	owners map[common.Hash][]uint64
	paths  map[string][]uint64

	layers     []*diffLayer
	taikoCache *taikoCache

	t *testing.T
}

func newTaikoTester(t *testing.T, taikoState uint64) *taikoTester {
	db, _ := rawdb.NewDatabaseWithFreezer(rawdb.NewMemoryDatabase(), "", "", false)
	return &taikoTester{
		db: db,
		taikoCache: newTaikoCache(&Config{
			TaikoState:     taikoState,
			CleanCacheSize: 100,
			DirtyCacheSize: 100,
		}, db),
		t: t,
	}
}

func fillLayers(startID uint64, count int) []*diffLayer {
	lyers := make([]*diffLayer, 0, count)
	for i := 0; i < count; i, startID = i+1, startID+1 {
		nodes := make(map[common.Hash]map[string]*trienode.Node)
		for j := 0; j < 20; j++ {
			var (
				path  = testutil.RandBytes(1)
				node  = testutil.RandomNode()
				owner = common.BigToHash(big.NewInt(int64(j)))
			)
			if _, ok := nodes[owner]; !ok {
				nodes[owner] = make(map[string]*trienode.Node)
			}
			nodes[owner][string(path)] = trienode.New(node.Hash, node.Blob)
		}
		lyers = append(lyers, &diffLayer{
			id:    startID,
			block: startID - 1,
			root:  testutil.RandomHash(),
			nodes: nodes,
		})
	}
	return lyers
}

func (t *taikoTester) close() {
	assert.NoError(t.t, t.taikoCache.Close())
	assert.NoError(t.t, t.db.Close())
}

func TestTaikoCache_recordLayers(t *testing.T) {
	var structTest = []struct {
		taikoState uint64
		fillCount  uint64
	}{
		{10, 5},
		{10, 10},
		{10, 15},
	}
	for _, val := range structTest {
		tester := newTaikoTester(t, val.taikoState)
		tester.layers = fillLayers(1, int(val.fillCount))

		blocks := make(map[uint64]common.Hash, val.fillCount)

		for _, layer := range tester.layers {
			blocks[layer.id] = layer.root
			assert.NoError(t, tester.taikoCache.recordDiffLayer(layer))
		}

		for id := uint64(1); id < val.fillCount; id++ {
			l := tester.taikoCache.Reader(blocks[id])
			data := rawdb.ReadNodeHistoryPrefix(tester.db, id)
			if val.taikoState < val.fillCount && id <= val.fillCount-val.taikoState {
				assert.Equal(t, 0, len(data))
				assert.Equal(t, nil, l)
			} else {
				assert.Equal(t, true, len(data) > 0, fmt.Sprintf("id: %d", id))
				assert.Equal(t, true, l != nil, fmt.Sprintf("id: %d", id))
			}
		}
		tester.close()
	}
}