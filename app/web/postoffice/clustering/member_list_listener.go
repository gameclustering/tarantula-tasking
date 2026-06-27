package clustering

import (
	"fmt"
	"strings"

	"gameclustering.com/internal/core"
	"github.com/hashicorp/memberlist"
)

const (
	REPLICA_RING_OPT uint32 = 1
	ALL_RING_OPT     uint32 = 3

	//SYNC_NODE_OPT uint32 = 8
	//SYNC_SUB_OPT  uint32 = 9

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

		}
	}
	core.AppLog.Info().Msg("local member list listener has stopped")
}
