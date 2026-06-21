package clustering

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

type MemberHashRing struct {
	NodeRing
	weight int
	WNode  chan<- RingUpdate
	hLock  *sync.Mutex
	auth   core.Authenticator
	caCert []byte
}

func (m *MemberHashRing) vNode(node core.Node, weight int) core.Node {
	v := core.Node{Name: fmt.Sprintf("%s#%d", node.Name, weight), IP: node.IP, RpcEndpoint: node.RpcEndpoint, Meta: node.Meta, State: node.State}
	v.RingToken = m.RingToken([]byte(v.Name))
	return v
}

func (m *MemberHashRing) OnAdd(node core.Node) {
	//core.AppLog.Debug().Msgf("node added rpc endpoint %s %s", node.RpcEndpoint, m.caCert)
	pool := core.RpcConnPool{Target: node.RpcEndpoint, Auth: m.auth, CACert: m.caCert}
	pool.Start()
	added := make([]core.Node, 0, m.weight)
	for w := range m.weight {
		v := m.vNode(node, w)
		v.CPool = &pool
		node.RingToken = m.RingToken([]byte(v.Name))
		m.nodes = append(m.nodes, v)
		added = append(added, v)
	}
	slices.SortFunc(m.nodes, cmp)
	m.nodeNum++
	update := RingUpdate{State: NODE_STATE_LIVE, Nodes: added}
	select {
	case m.WNode <- update:
	default:
		go func() { m.WNode <- update }()
	}
}

func (m *MemberHashRing) OnRemove(node core.Node) {
	removed := make([]core.Node, 0, m.weight)
	m.nodes = slices.DeleteFunc(m.nodes, func(n core.Node) bool {
		if n.IP == node.IP {
			n.State = NODE_STATE_DEAD
			removed = append(removed, n)
			return true
		}
		return false
	})
	slices.SortFunc(m.nodes, cmp)
	m.nodeNum--
	core.AppLog.Debug().Msgf("remove meta %s", removed[0].Meta)
	mpart := strings.Split(removed[0].Meta, ":")
	removed[0].CPool.Tag = mpart[0]
	removed[0].CPool.NodeId = mpart[1]
	removed[0].CPool.Release()
	update := RingUpdate{State: NODE_STATE_DEAD, Nodes: removed}
	select {
	case m.WNode <- update:
	default:
		go func() { m.WNode <- update }()
	}
}

func (m *MemberHashRing) OnUpdate(node core.Node) {
	//core.AppLog.Debug().Msgf("node updated rpc endpoint %s %s", node.RpcEndpoint, m.caCert)
	for i, n := range m.nodes {
		if strings.HasPrefix(n.Name, node.Name) {
			n.Meta = node.Meta
			m.nodes[i] = n
		}
	}
}

func (m *MemberHashRing) OnMerge(nodes []core.Node) {
	core.AppLog.Debug().Msgf("merging request nodes %d", len(nodes))
}

func (m *MemberHashRing) OnLive(node core.Node) {

}

func (m *MemberHashRing) OnPing(node core.Node) {

}

func (m *MemberHashRing) OnConflict(nodes []core.Node) {

}

// hash ring operations
func (m *MemberHashRing) RingToken(key []byte) uint32 {
	m.hLock.Lock()
	defer m.hLock.Unlock()
	return util.Hash32(key)
}
