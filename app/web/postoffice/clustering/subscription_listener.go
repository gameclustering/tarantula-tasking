package clustering

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
	"github.com/hashicorp/memberlist"
)

type SubscriptionListener struct {
	MRequest chan core.RingRequest
	MSync    chan<- []byte
	*memberlist.Memberlist
	*MemberHashRing
	//*DataServiceProvider
	//meta []byte
}

func (m *SubscriptionListener) Listen() {
	running := true
	for running {
		mr := <-m.MRequest
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
		}
	}
	core.AppLog.Info().Msg("local subscription listener has stopped")
}
