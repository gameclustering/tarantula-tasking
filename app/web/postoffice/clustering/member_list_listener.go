package clustering

import (
	"fmt"
	"strings"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
	"github.com/hashicorp/memberlist"
)

const (
	REPLICA_RING_OPT uint32 = 1
	ALL_RING_OPT     uint32 = 3

	SYNC_NODE_OPT uint32 = 8
	SYNC_SUB_OPT  uint32 = 9

	CLOSE_RING_OPT uint32 = 99

	NODE_STATE_LIVE     = 0
	NODE_STATE_DEAD     = 3
	NODE_STATE_SHUTDOWN = -1000
)

type RetryTrack struct {
	Err    string
	Reties int
	Suc    bool
}

type MemberListListener struct {
	MEvent    chan memberlist.NodeEvent
	MMerge    chan []core.Node
	MAlive    chan core.Node
	MPing     chan core.Node
	MConflict chan []core.Node
	MRequest  chan core.RingRequest
	MSync     chan<- []byte
	*memberlist.Memberlist
	*MemberHashRing
	*DataServiceProvider
	meta []byte
}

func (m *MemberListListener) toNode(e *memberlist.Node) core.Node {
	parts := strings.Split(e.Address(), ":")
	return core.Node{Name: e.Name, Meta: string(e.Meta), IP: e.Address(), RpcEndpoint: fmt.Sprintf("%s:%d", parts[0], core.RPC_PORT), State: int(e.State)}
}

// event dispatch from event delegate
func (m *MemberListListener) Listen() {
	running := true
	for running {
		select {
		case e := <-m.MEvent:
			switch e.Event {
			case memberlist.NodeJoin:
				core.AppLog.Debug().Msgf("META ADDED %s", string(e.Node.Meta))
				m.OnAdd(m.toNode(e.Node))
			case memberlist.NodeLeave:
				if (m.LocalNode().Name) != e.Node.Name {
					m.OnRemove(m.toNode(e.Node))
				}
			case memberlist.NodeUpdate:
				core.AppLog.Debug().Msgf("META UPDATED %s", string(e.Node.Meta))
				m.OnUpdate(m.toNode(e.Node))
			}
		case mg := <-m.MMerge:
			m.OnMerge(mg)
		case ma := <-m.MAlive:
			m.OnLive(ma)
		case mp := <-m.MPing:
			m.OnPing(mp)
		case mc := <-m.MConflict:
			m.OnConflict(mc)
		case mr := <-m.MRequest:
			switch mr.Opt {
			case REPLICA_RING_OPT:
				nodes := m.keyRing(mr.Token, mr.Replicas)
				mr.Async <- nodes
			case ALL_RING_OPT:
				nodes := make([]core.Node, 0)
				for _, n := range m.nodes {
					nodes = append(nodes, n)
				}
				mr.Async <- nodes
			case SYNC_NODE_OPT:
				for _, mbr := range m.Members() {
					if mbr.Address() == mr.Address {
						core.AppLog.Debug().Msgf("sending sync message to %s", mr.Address)
						m.SendToAddress(mbr.FullAddress(), util.ToJson(mr.Source))
						break
					}
				}
			case SYNC_SUB_OPT:
				if mr.Address != "" {
					for _, mbr := range m.Members() {
						if mbr.Address() == mr.Address {
							core.AppLog.Debug().Msgf("sending topic message to %s", mbr.FullAddress().Name)
							m.SendReliable(mbr, util.ToJson(mr.Source))
							break
						}
					}
				} else {
					localAddr := m.LocalNode().Address()
					for _, mbr := range m.Members() {
						if mbr.Address() == localAddr {
							continue // skip self — already registered via direct MSync in Subscribe
						}
						core.AppLog.Debug().Msgf("sending topic message to %s", mbr.FullAddress().Name)
						m.SendToAddress(mbr.FullAddress(), util.ToJson(mr.Source))
					}
				}
			case CLOSE_RING_OPT:
				running = false
			}
		}
	}
	core.AppLog.Info().Msg("local member listener has stopped")
}

func (m *MemberListListener) rangeRing(r core.RingRequest) {
	m.MRequest <- r
}
func (m *MemberListListener) localNode(node core.Node) bool {
	return strings.HasPrefix(node.Name, m.LocalNode().Name)
}

// delegate
func (m *MemberListListener) NodeMeta(limit int) []byte {
	//limit 512
	return m.meta
}

func (m *MemberListListener) NotifyMsg(msg []byte) {
	select {
	case m.MSync <- msg:
	default:
		core.AppLog.Warn().Msgf("NotifyMsg: MSync full, dropping subscription sync message")
	}
}

func (m *MemberListListener) GetBroadcasts(overhead, limit int) [][]byte {
	//overhead 3 limit 1350
	return nil
}

func (m *MemberListListener) LocalState(join bool) []byte {
	return nil
}
func (m *MemberListListener) MergeRemoteState(buf []byte, join bool) {

}

// ping delegate
func (m *MemberListListener) AckPayload() []byte {
	return nil
}

func (m *MemberListListener) NotifyPingComplete(other *memberlist.Node, rtt time.Duration, payload []byte) {
	m.MPing <- m.toNode(other)
}

// merge delegate
func (m *MemberListListener) NotifyMerge(peers []*memberlist.Node) error {
	nodes := make([]core.Node, 0, len(peers))
	for _, n := range peers {
		nodes = append(nodes, m.toNode(n))
	}
	m.MMerge <- nodes
	return nil
}

// alive delegate
func (m *MemberListListener) NotifyAlive(peer *memberlist.Node) error {
	m.MAlive <- m.toNode(peer)
	return nil
}

// conflict delegate
func (m *MemberListListener) NotifyConflict(existing, other *memberlist.Node) {
	m.MConflict <- []core.Node{m.toNode(existing), m.toNode(other)}
}
