package clustering

import (
	"fmt"
	"strings"

	"gameclustering.com/internal/core"
	//"gameclustering.com/internal/util"
	"github.com/hashicorp/memberlist"
)

const (
	REPLICA_RING_OPT uint32 = 1
	ALL_RING_OPT     uint32 = 3

	SYNC_NODE_OPT uint32 = 8
	SYNC_SUB_OPT  uint32 = 9

	//CLOSE_RING_OPT uint32 = 99

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
	mEvent    chan memberlist.NodeEvent
	mMerge    chan []core.Node
	mAlive    chan core.Node
	mPing     chan core.Node
	mConflict chan []core.Node
	//MRequest  chan core.RingRequest
	//MSync     chan<- []byte
	*memberlist.Memberlist
	memberListChangeListener MemberListChangeListener
	meta                     []byte
}

func (m *MemberListListener) toNode(e *memberlist.Node) core.Node {
	parts := strings.Split(e.Address(), ":")
	return core.Node{Name: e.Name, Meta: string(e.Meta), IP: e.Address(), RpcEndpoint: fmt.Sprintf("%s:%d", parts[0], core.RPC_PORT), State: int(e.State)}
}

// event dispatch from event delegate
func (m *MemberListListener) Listen() {
running:
	for {
		select {
		case e, ok := <-m.mEvent:
			if !ok {
				break running
			}
			switch e.Event {
			case memberlist.NodeJoin:
				core.AppLog.Debug().Msgf("META ADDED %s", string(e.Node.Meta))
				m.memberListChangeListener.NodeAdded(m.toNode(e.Node))
			case memberlist.NodeLeave:
				if (m.LocalNode().Name) != e.Node.Name {
					m.memberListChangeListener.NodeRemoved(m.toNode(e.Node))
				}
			case memberlist.NodeUpdate:
				core.AppLog.Debug().Msgf("META UPDATED %s", string(e.Node.Meta))
				m.memberListChangeListener.NodeUpdated(m.toNode(e.Node))
			}
		case mg, ok := <-m.mMerge:
			if !ok {
				break running
			}
			m.memberListChangeListener.NodesMerged(mg)
		case ma, ok := <-m.mAlive:
			if !ok {
				break running
			}
			m.memberListChangeListener.NodeLived(ma)
		case mp, ok := <-m.mPing:
			if !ok {
				break running
			}
			m.memberListChangeListener.NodePinged(mp)
		case mc, ok := <-m.mConflict:
			if !ok {
				break running
			}
			m.memberListChangeListener.NodesConflicted(mc)
			/**
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
						// Pre-serialize once; all goroutines read the same bytes (no write after this).
						payload := util.ToJson(mr.Source)
						for _, mbr := range m.Members() {
							if mbr.Address() == localAddr {
								continue // skip self — already registered via direct MSync in Subscribe
							}
							core.AppLog.Debug().Msgf("sending topic message to %s", mbr.FullAddress().Name)
							// Use SendReliable (TCP) in a goroutine so Listen() is not blocked
							// and subscriptions are delivered even under transient UDP loss.
							go m.SendReliable(mbr, payload)
						}
					}
				case CLOSE_RING_OPT:
					running = false
				}**/
		}
	}
	core.AppLog.Info().Msg("local member list listener has stopped")
}

func (m *MemberListListener) rangeRing(r core.RingRequest) {
	//m.MRequest <- r
}
func (m *MemberListListener) localNode(node core.Node) bool {
	return strings.HasPrefix(node.Name, m.LocalNode().Name)
}
