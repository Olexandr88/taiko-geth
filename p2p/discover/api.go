package discover

import (
	"errors"

	"github.com/ethereum/go-ethereum/p2p/enode"
)

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
	Buckets     []string `json:"buckets"`
	LocalNodeId string   `json:"localNodeId"`
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

	closestNodes := d.DiscV5.AllNodes()
	buckets := make([]string, len(closestNodes))
	for _, e := range closestNodes {
		buckets = append(buckets, e.ID().String())
	}

	return &RoutingTableInfo{
		Buckets:     buckets,
		LocalNodeId: n.ID().String(),
	}
}

func (d *DiscV5API) AddEnr(enr string) (bool, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return false, err
	}

	d.DiscV5.tab.addSeenNode(wrapNode(n))
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

func (p *PortalAPI) RoutingTableInfo() *RoutingTableInfo {
	n := p.portalProtocol.localNode.Node()

	closestNodes := p.portalProtocol.table.Nodes()
	buckets := make([]string, len(closestNodes))
	for _, e := range closestNodes {
		buckets = append(buckets, e.ID().String())
	}

	return &RoutingTableInfo{
		Buckets:     buckets,
		LocalNodeId: n.ID().String(),
	}
}

func (p *PortalAPI) AddEnr(enr string) (bool, error) {
	n, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return false, err
	}

	p.portalProtocol.table.addSeenNode(wrapNode(n))
	return true, nil
}

func (p *PortalAPI) AddEnrs(enrs []string) bool {
	// Note: unspecified RPC, but useful for our local testnet test
	for _, enr := range enrs {
		n, err := enode.Parse(enode.ValidSchemes, enr)
		if err != nil {
			continue
		}

		p.portalProtocol.table.addSeenNode(wrapNode(n))
	}

	return true
}

func (p *PortalAPI) GetEnr(nodeId string) (string, error) {
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

func (p *PortalAPI) DeleteEnr(nodeId string) bool {
	id, err := enode.ParseID(nodeId)
	if err != nil {
		return false
	}

	n := p.portalProtocol.table.getNode(id)
	if n == nil {
		return false
	}

	p.portalProtocol.table.delete(wrapNode(n))
	return true
}

func (p *PortalAPI) LookupEnr(nodeId string) (string, error) {
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
