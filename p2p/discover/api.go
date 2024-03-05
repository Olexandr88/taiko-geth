package discover

import (
	"errors"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/p2p/discover/portalwire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/portalnetwork/storage"
	"github.com/holiman/uint256"
)

// json-rpc spec
// https://playground.open-rpc.org/?schemaUrl=https://raw.githubusercontent.com/ethereum/portal-network-specs/assembled-spec/jsonrpc/openrpc.json&uiSchema%5BappBar%5D%5Bui:splitView%5D=false&uiSchema%5BappBar%5D%5Bui:input%5D=false&uiSchema%5BappBar%5D%5Bui:examplesDropdown%5D=false
type DiscV5API struct {
	DiscV5 *UDPv5
}

func NewAPI(discV5 *UDPv5) *DiscV5API {
	return &DiscV5API{discV5}
}

type NodeInfo struct {
	NodeId string `json:"nodeId"`
	Enr    string `json:"enr"`
	Ip     string `json:"ip"`
}

type RoutingTableInfo struct {
	Buckets     [][]string `json:"buckets"`
	LocalNodeId string     `json:"localNodeId"`
}

type DiscV5PongResp struct {
	EnrSeq        uint64 `json:"enrSeq"`
	RecipientIP   string `json:"recipientIP"`
	RecipientPort uint16 `json:"recipientPort"`
}

type PortalPongResp struct {
	EnrSeq     uint32 `json:"enrSeq"`
	DataRadius string `json:"dataRadius"`
}

type ContentInfo struct {
	Content     string `json:"content"`
	UtpTransfer bool   `json:"utpTransfer"`
}

type Enrs struct {
	Enrs []string `json:"enrs"`
}

func (d *DiscV5API) NodeInfo() *NodeInfo {
	n := d.DiscV5.LocalNode().Node()

	return &NodeInfo{
		NodeId: n.ID().String(),
		Enr:    n.String(),
		Ip:     n.IP().String(),
	}
}

func (d *DiscV5API) RoutingTableInfo() *RoutingTableInfo {
	n := d.DiscV5.LocalNode().Node()

	return &RoutingTableInfo{
		Buckets:     d.DiscV5.RoutingTableInfo(),
		LocalNodeId: n.ID().String(),
	}
}

func (d *DiscV5API) AddEnr(enr string) (bool, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return false, err
	}

	wn := wrapNode(n)
	wn.livenessChecks++
	d.DiscV5.tab.addVerifiedNode(wn)
	return true, nil
}

func (d *DiscV5API) GetEnr(nodeId string) (bool, error) {
	id, err := enode.ParseID(nodeId)
	if err != nil {
		return false, err
	}
	n := d.DiscV5.tab.getNode(id)
	if n == nil {
		return false, errors.New("record not in local routing table")
	}

	return true, nil
}

func (d *DiscV5API) DeleteEnr(nodeId string) (bool, error) {
	id, err := enode.ParseID(nodeId)
	if err != nil {
		return false, err
	}

	n := d.DiscV5.tab.getNode(id)
	if n == nil {
		return false, errors.New("record not in local routing table")
	}

	d.DiscV5.tab.delete(wrapNode(n))
	return true, nil
}

func (d *DiscV5API) LookupEnr(nodeId string) (string, error) {
	id, err := enode.ParseID(nodeId)
	if err != nil {
		return "", err
	}

	enr := d.DiscV5.ResolveNodeId(id)

	if enr == nil {
		return "", errors.New("record not found in DHT lookup")
	}

	return enr.String(), nil
}

func (d *DiscV5API) Ping(enr string) (*DiscV5PongResp, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return nil, err
	}

	pong, err := d.DiscV5.pingInner(n)
	if err != nil {
		return nil, err
	}

	return &DiscV5PongResp{
		EnrSeq:        pong.ENRSeq,
		RecipientIP:   pong.ToIP.String(),
		RecipientPort: pong.ToPort,
	}, nil
}

func (d *DiscV5API) FindNodes(enr string, distances []uint) ([]string, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return nil, err
	}
	findNodes, err := d.DiscV5.findnode(n, distances)
	if err != nil {
		return nil, err
	}

	enrs := make([]string, 0, len(findNodes))
	for _, r := range findNodes {
		enrs = append(enrs, r.String())
	}

	return enrs, nil
}

func (d *DiscV5API) TalkReq(enr string, protocol string, payload string) (string, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return "", err
	}

	req, err := hexutil.Decode(payload)
	if err != nil {
		return "", err
	}

	talkResp, err := d.DiscV5.TalkRequest(n, protocol, req)
	if err != nil {
		return "", err
	}
	return hexutil.Encode(talkResp), nil
}

func (d *DiscV5API) RecursiveFindNodes(nodeId string) ([]string, error) {
	findNodes := d.DiscV5.Lookup(enode.HexID(nodeId))

	enrs := make([]string, 0, len(findNodes))
	for _, r := range findNodes {
		enrs = append(enrs, r.String())
	}

	return enrs, nil
}

type PortalAPI struct {
	*DiscV5API
	portalProtocol *PortalProtocol
}

func NewPortalAPI(portalProtocol *PortalProtocol) *PortalAPI {
	return &PortalAPI{
		DiscV5API:      &DiscV5API{portalProtocol.DiscV5},
		portalProtocol: portalProtocol,
	}
}

func (p *PortalAPI) NodeInfo() *NodeInfo {
	n := p.portalProtocol.localNode.Node()

	return &NodeInfo{
		NodeId: n.ID().String(),
		Enr:    n.String(),
		Ip:     n.IP().String(),
	}
}

func (p *PortalAPI) HistoryRoutingTableInfo() *RoutingTableInfo {
	n := p.portalProtocol.localNode.Node()

	return &RoutingTableInfo{
		Buckets:     p.portalProtocol.RoutingTableInfo(),
		LocalNodeId: n.ID().String(),
	}
}

func (p *PortalAPI) HistoryAddEnr(enr string) (bool, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return false, err
	}

	wn := wrapNode(n)
	wn.livenessChecks++
	p.portalProtocol.table.addVerifiedNode(wn)
	return true, nil
}

func (p *PortalAPI) AddEnrs(enrs []string) bool {
	// Note: unspecified RPC, but useful for our local testnet test
	for _, enr := range enrs {
		n, err := enode.Parse(enode.ValidSchemes, enr)
		if err != nil {
			continue
		}

		wn := wrapNode(n)
		wn.livenessChecks++
		p.portalProtocol.table.addVerifiedNode(wn)
	}

	return true
}

func (p *PortalAPI) HistoryGetEnr(nodeId string) (string, error) {
	id, err := enode.ParseID(nodeId)
	if err != nil {
		return "", err
	}

	if id == p.portalProtocol.localNode.Node().ID() {
		return p.portalProtocol.localNode.Node().String(), nil
	}

	n := p.portalProtocol.table.getNode(id)
	if n == nil {
		return "", errors.New("record not in local routing table")
	}

	return n.String(), nil
}

func (p *PortalAPI) HistoryDeleteEnr(nodeId string) (bool, error) {
	id, err := enode.ParseID(nodeId)
	if err != nil {
		return false, err
	}

	n := p.portalProtocol.table.getNode(id)
	if n == nil {
		return false, nil
	}

	p.portalProtocol.table.delete(wrapNode(n))
	return true, nil
}

func (p *PortalAPI) HistoryLookupEnr(nodeId string) (string, error) {
	id, err := enode.ParseID(nodeId)
	if err != nil {
		return "", err
	}

	enr := p.portalProtocol.ResolveNodeId(id)

	if enr == nil {
		return "", errors.New("record not found in DHT lookup")
	}

	return enr.String(), nil
}

func (p *PortalAPI) HistoryPing(enr string) (*PortalPongResp, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return nil, err
	}

	pong, err := p.portalProtocol.pingInner(n)
	if err != nil {
		return nil, err
	}

	customPayload := &portalwire.PingPongCustomData{}
	err = customPayload.UnmarshalSSZ(pong.CustomPayload)
	if err != nil {
		return nil, err
	}

	nodeRadius := new(uint256.Int)
	err = nodeRadius.UnmarshalSSZ(customPayload.Radius)
	if err != nil {
		return nil, err
	}

	return &PortalPongResp{
		EnrSeq:     uint32(pong.EnrSeq),
		DataRadius: nodeRadius.Hex(),
	}, nil
}

func (p *PortalAPI) HistoryFindNodes(enr string, distances []uint) ([]string, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return nil, err
	}
	findNodes, err := p.portalProtocol.findNodes(n, distances)
	if err != nil {
		return nil, err
	}

	enrs := make([]string, 0, len(findNodes))
	for _, r := range findNodes {
		enrs = append(enrs, r.String())
	}

	return enrs, nil
}

func (p *PortalAPI) HistoryFindContent(enr string, contentKey string) (interface{}, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return nil, err
	}

	contentKeyBytes, err := hexutil.Decode(contentKey)
	if err != nil {
		return nil, err
	}

	flag, findContent, err := p.portalProtocol.findContent(n, contentKeyBytes)
	if err != nil {
		return nil, err
	}

	switch flag {
	case portalwire.ContentRawSelector:
		contentInfo := &ContentInfo{
			Content:     hexutil.Encode(findContent.([]byte)),
			UtpTransfer: false,
		}
		p.portalProtocol.log.Trace("HistoryFindContent", "contentInfo", contentInfo)
		return contentInfo, nil
	case portalwire.ContentConnIdSelector:
		contentInfo := &ContentInfo{
			Content:     hexutil.Encode(findContent.([]byte)),
			UtpTransfer: true,
		}
		p.portalProtocol.log.Trace("HistoryFindContent", "contentInfo", contentInfo)
		return contentInfo, nil
	default:
		enrs := make([]string, 0)
		for _, r := range findContent.([]*enode.Node) {
			enrs = append(enrs, r.String())
		}

		p.portalProtocol.log.Trace("HistoryFindContent", "enrs", enrs)
		return &Enrs{
			Enrs: enrs,
		}, nil
	}
}

func (p *PortalAPI) HistoryOffer(enr string, contentKey string, contentValue string) (string, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return "", err
	}

	contentKeyBytes, err := hexutil.Decode(contentKey)
	if err != nil {
		return "", err
	}
	contentValueBytes, err := hexutil.Decode(contentValue)
	if err != nil {
		return "", err
	}

	contentEntry := &ContentEntry{
		ContentKey: contentKeyBytes,
		Content:    contentValueBytes,
	}

	transientOfferRequest := &TransientOfferRequest{
		Contents: []*ContentEntry{contentEntry},
	}

	offerReq := &OfferRequest{
		Kind:    TransientOfferRequestKind,
		Request: transientOfferRequest,
	}
	accept, err := p.portalProtocol.offer(n, offerReq)
	if err != nil {
		return "", err
	}

	return hexutil.Encode(accept), nil
}

func (p *PortalAPI) HistoryRecursiveFindNodes(nodeId string) ([]string, error) {
	findNodes := p.portalProtocol.Lookup(enode.HexID(nodeId))

	enrs := make([]string, 0, len(findNodes))
	for _, r := range findNodes {
		enrs = append(enrs, r.String())
	}

	return enrs, nil
}

func (p *PortalAPI) HistoryRecursiveFindContent(contentKeyHex string) (*ContentInfo, error) {
	contentKey, err := hexutil.Decode(contentKeyHex)
	if err != nil {
		return nil, err
	}
	content, utpTransfer, err := p.portalProtocol.ContentLookup(contentKey)
	if errors.Is(err, storage.ErrContentNotFound) {
		return &ContentInfo{
			Content:     "0x",
			UtpTransfer: false,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return &ContentInfo{
		Content:     hexutil.Encode(content),
		UtpTransfer: utpTransfer,
	}, err
}

func (p *PortalAPI) HistoryLocalContent(contentKeyHex string) (string, error) {
	contentKey, err := hexutil.Decode(contentKeyHex)
	if err != nil {
		return "", err
	}
	contentId := p.portalProtocol.ToContentId(contentKey)
	content, err := p.portalProtocol.Get(contentId)
	if errors.Is(err, storage.ErrContentNotFound) {
		return "0x", nil
	}
	if err != nil {
		return "", err
	}
	return hexutil.Encode(content), nil
}

func (p *PortalAPI) HistoryStore(contentKeyHex string, contextHex string) (bool, error) {
	contentKey, err := hexutil.Decode(contentKeyHex)
	if err != nil {
		return false, err
	}
	contentId := p.portalProtocol.ToContentId(contentKey)
	if !p.portalProtocol.InRange(contentId) {
		return false, nil
	}
	content, err := hexutil.Decode(contextHex)
	if err != nil {
		return false, err
	}
	err = p.portalProtocol.Put(contentId, content)
	if err != nil {
		return false, err
	}
	return true, nil
}

// TODO
func (p *PortalAPI) HistoryGossip(contentKeyHex, contentHex string) (int, error) {
	contentKey, err := hexutil.Decode(contentKeyHex)
	if err != nil {
		return 0, err
	}
	content, err := hexutil.Decode(contentHex)
	if err != nil {
		return 0, err
	}
	id := p.portalProtocol.Self().ID()
	return p.portalProtocol.NeighborhoodGossip(&id, [][]byte{contentKey}, [][]byte{content})
}

// TODO
func (p *PortalAPI) HistoryTraceRecursiveFindContent(contentKeyHex string) {

}