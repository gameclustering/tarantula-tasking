package clustering

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	RECEIVER_START  uint32 = 1
	TOPIC_REGISTER  uint32 = 2
	RECEIVER_REMOVE uint32 = 3
	RECEIVER_END    uint32 = 4
	TASK_REGISTER   uint32 = 5

	TOPIC_LIST uint32 = 6
	TASK_LIST  uint32 = 7

	// TASK_ASSIGN picks one subscriber via round-robin scoped by tag.
	// Reserve and confirm phases call this independently so they can land
	// on different nodes.
	TASK_ASSIGN uint32 = 8

	TRANS_SUB_PREFIX string = "_t_"

	NODE_ADDED   uint32 = 0
	NODE_REMOVED uint32 = 1
	NODE_UPDATED uint32 = 3
)

type ReceiverAsync struct {
	Rev  chan *protocol.Mail
	Q    chan string
	Subs map[string]core.Subscription
}

type MemberHashRingListener struct {
	*DataServiceProvider
}

type NodeRequest struct {
	opt  uint32
	node core.Node
}

type RegisterRequest struct {
	opt uint32
	sub core.Subscription
}

type TopicRequest struct {
	Opt    uint32
	NodeId string
	Tag    string
	Name   string
	// QChan is the Q channel of the Receive goroutine sending RECEIVER_REMOVE.
	// The dispatcher only deletes the listener when this matches the current entry,
	// preventing stale goroutines from evicting a freshly-registered listener.
	QChan chan string

	Async chan ReceiverAsync
	Subs  chan []core.Subscription
}

func (m *MemberHashRingListener) balanceOnNodeAdded(added []core.Node) {

	if m.backRing.nodeNum == 0 {
		m.backRing.nodes = append(m.backRing.nodes, added...)
		slices.SortFunc(m.backRing.nodes, cmp)
		m.backRing.nodeNum++
		return
	}
	slices.SortFunc(added, cmp)
	ringSync := core.RingSync{Ranges: make([]core.RingRange, 0)}
	for _, n := range added {
		if !m.localNode(n) { //skip node initial add call
			ringRange := m.backRing.rangeOfRing(n.RingToken)
			if m.localNode(ringRange[1]) {
				ringSync.Remote = ringRange[1].RpcEndpoint
				ringSync.Ranges = append(ringSync.Ranges, core.RingRange{From: ringRange[0].RingToken, To: n.RingToken})
				core.AppLog.Debug().Msgf("push data key hash >= %d and < %d to remote node %s", ringRange[0].RingToken, n.RingToken, n.IP)
			}
		}
		m.backRing.nodes = append(m.backRing.nodes, n)
		slices.SortFunc(m.backRing.nodes, cmp)
	}
	m.backRing.nodeNum++
	// Data range handoff only when this node owns ranges adjacent to the new node.

	if len(ringSync.Ranges) > 0 {
		nodeReq := core.RingRequest{Source: ringSync, Opt: SYNC_NODE_OPT, Address: added[0].IP}
		select {
		case m.MRequest <- nodeReq:
		default:
			go func() { m.MRequest <- nodeReq }()
		}
	}
	// Subscription sync is always needed: every node must push its subscriptions
	// to the new node so it can route tasks correctly, regardless of ring ranges.
	subsync := protocol.SubsSync{Subs: make([]*protocol.Subscription, 0)}
	m.subscriptions.lookup(func(sub core.Subscription) {
		if sub.Endpoint == m.rpcEndpoint {
			subsync.Subs = append(subsync.Subs, sub.ToProto())
		}
	})
	if len(subsync.Subs) > 0 {
		go m.runSyncSubs(&subsync)
	}
}

func (m *MemberHashRingListener) balanceOnNodeRemoved(removed []core.Node) {

	for _, n := range removed {
		m.backRing.nodes = slices.DeleteFunc(m.backRing.nodes, func(d core.Node) bool {
			return d.IP == n.IP
		})
	}
	slices.SortFunc(m.backRing.nodes, cmp)
	m.backRing.nodeNum--
	m.subscriptions.lookup(func(sub core.Subscription) {
		if sub.Endpoint == removed[0].RpcEndpoint {
			m.subscriptions.del(sub)
		}
	})
}

func (m *MemberHashRingListener) registerSubscription(sub core.Subscription) {
	if sub.Type == core.TRANS_MAIL && !strings.HasPrefix(sub.Topic, TRANS_SUB_PREFIX) {
		sub.Topic = fmt.Sprintf("%s%s", TRANS_SUB_PREFIX, sub.Topic)
	}
	if sub.Deleting {
		m.subscriptions.del(sub)
		// Only local subscriptions have listener entries; remote ones are routing-only.
		if sub.Endpoint == m.rpcEndpoint {
			if listener, ok := m.listeners[sub.NodeId]; ok {
				delete(listener.Subs, sub.Topic)
			}
		}
		return
	}
	m.subscriptions.add(sub)
	// Gossiped subscriptions from remote postoffices are for pick() routing only.
	// Creating phantom Rev channels for them causes TRANS_MAIL to be silently
	// dropped into unread channels when a remote worker's entry appears first in
	// the listenerPool before the locally-connected worker.
	if sub.Endpoint != m.rpcEndpoint {
		return
	}
	listener, ok := m.listeners[sub.NodeId]
	if !ok {
		listener = ReceiverAsync{Rev: make(chan *protocol.Mail, NODE_EVENT_BUFFER_SIZE), Q: make(chan string, 2), Subs: make(map[string]core.Subscription)}
		m.listeners[sub.NodeId] = listener
		m.listenerPool = append(m.listenerPool, sub.NodeId)
	}
	listener.Subs[sub.Topic] = sub
}

func (m *MemberHashRingListener) RingUpdated() {
running:
	for {
		select {
		case nr, ok := <-m.nRequest:
			if !ok {
				break running
			}
			switch nr.opt {
			case NODE_ADDED:
				m.balanceOnNodeAdded(m.OnAdd(nr.node))
			case NODE_REMOVED:
				m.balanceOnNodeRemoved(m.OnRemove(nr.node))
			case NODE_UPDATED:
				m.OnUpdate(nr.node)
			}
		case reg, ok := <-m.sRquest:
			if !ok {
				break running
			}
			m.registerSubscription(reg.sub)
		case sync, ok := <-m.RSync:
			if !ok {
				break running
			}
			var ds core.RingSync
			err := json.Unmarshal(sync, &ds)
			if err != nil {
				core.AppLog.Warn().Msgf("cannot parse remote data from %s", string(sync))
			} else {
				if len(ds.Ranges) > 0 {
					// Run recovery in a background goroutine so DSet workers
					// remain available for normal Create/Update requests.
					go m.recoverFromNode(ds)
				} else {
					m.registerSubscription(ds.Sub)
				}
			}
		case mr, ok := <-m.MRequest:
			if !ok {
				break running
			}
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
			}
		case req, ok := <-m.DRequest:
			if !ok {
				break running
			}
			switch req.Opt {
			case RECEIVER_START:
				_, existed := m.listeners[req.Name]
				// Always allocate a fresh channel pair so each Receive goroutine
				// has exclusive ownership — prevents double-close on reconnect.
				rev := ReceiverAsync{Rev: make(chan *protocol.Mail, NODE_EVENT_BUFFER_SIZE), Q: make(chan string, 2), Subs: make(map[string]core.Subscription)}
				m.listeners[req.Name] = rev
				if !existed {
					m.listenerPool = append(m.listenerPool, req.Name)
				}
				req.Async <- rev
			case RECEIVER_END:
				rev, ok := m.listeners[req.Name]
				if ok {
					rev.Q <- req.Name
				}

			case RECEIVER_REMOVE:
				// Only delete if the Q channel matches — stale goroutines must not
				// evict a freshly-registered listener created during reconnect.
				if current, ok := m.listeners[req.Name]; ok && current.Q == req.QChan {
					delete(m.listeners, req.Name)
				}
			case TOPIC_REGISTER:
				req.Subs <- m.subscriptions.topic(req)
			case TASK_REGISTER:
				req.Name = fmt.Sprintf("%s%s", TRANS_SUB_PREFIX, req.Name)
				req.Subs <- m.subscriptions.topic(req)
			case TASK_ASSIGN:
				name := fmt.Sprintf("%s%s", TRANS_SUB_PREFIX, req.Name)
				var sub *core.Subscription
				if req.NodeId != "" {
					sub = m.subscriptions.pickByNodeId(name, req.NodeId)
				}
				if sub == nil {
					sub = m.subscriptions.pick(name, req.Tag)
				}
				if sub != nil {
					req.Subs <- []core.Subscription{*sub}
				} else {
					req.Subs <- []core.Subscription{}
				}
			case TOPIC_LIST:
				req.Subs <- m.subscriptions.list(false)
			case TASK_LIST:
				req.Subs <- m.subscriptions.list(true)
			}
		case msg, ok := <-m.DMessager:
			if !ok {
				break running
			}
			switch msg.Opt {
			case core.TOPIC_MAIL:
				for _, ch := range m.listeners {
					_, subed := ch.Subs[msg.Topic.Name]
					if subed {
						select {
						case ch.Rev <- msg:
						default:
							core.AppLog.Warn().Msgf("TOPIC_MAIL dropped (Rev full) topic=%s", msg.Topic.Name)
						}
					}
				}
			case core.TRANS_MAIL:
				tn := fmt.Sprintf("%s%s", TRANS_SUB_PREFIX, msg.Transaction.Meta.Name)
				lk := ""
				delivered := false
				for {
					if len(m.listenerPool) == 0 {
						break
					}
					nk := m.listenerPool[0]
					if lk == nk {
						break
					}
					nc, exists := m.listeners[nk]
					if !exists {
						m.listenerPool = m.listenerPool[1:] //remove key if disconnected
						continue
					}
					_, subed := nc.Subs[tn]
					if subed {
						select {
						case nc.Rev <- msg:
							core.AppLog.Info().Msgf("TRANS_MAIL delivered txn=%d name=%s to=%s", msg.Transaction.Meta.Id, msg.Transaction.Meta.Name, nk)
							delivered = true
						default:
							core.AppLog.Warn().Msgf("TRANS_MAIL dropped (Rev full) txn=%d name=%s to=%s", msg.Transaction.Meta.Id, msg.Transaction.Meta.Name, nk)
						}
						m.listenerPool = append(m.listenerPool[1:], nk) //add to tail
						break
					}
					//mark last one to break loop if fullly iterated
					lk = nk
					m.listenerPool = append(m.listenerPool[1:], nk) //add to tail
				}
				if !delivered {
					core.AppLog.Warn().Msgf("TRANS_MAIL dropped txn=%d name=%s pool=%d", msg.Transaction.Meta.Id, msg.Transaction.Meta.Name, len(m.listenerPool))
				}
			}
		}

	}
	//shutdown server
	close(m.shutdown)
	for range SET_OPERATOR_NUM {
		m.DSet <- SetData{Opt: core.SET_OPT_CLOSE}
	}
	close(m.DSet)
	m.server.Stop()
	m.Local.Close()
	core.AppLog.Info().Msg("local member hash ring listener has stopped")
}

// recoverFromNode pulls ring-partition data from ds.Remote in the background.
// Runs in its own goroutine so DSet workers stay available for normal writes.
func (m *MemberHashRingListener) recoverFromNode(ds core.RingSync) {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(m.CACert)
	creds := credentials.NewTLS(&tls.Config{RootCAs: pool})
	p := core.RpcConnPool{Auth: m.auth}
	tcp, err := grpc.NewClient(ds.Remote,
		grpc.WithTransportCredentials(creds),
		grpc.WithUnaryInterceptor(p.OnCall),
		grpc.WithStreamInterceptor(p.OnStreaming),
	)
	if err != nil {
		core.AppLog.Warn().Msgf("recovery connect error from %s: %s", ds.Remote, err.Error())
		return
	}
	defer tcp.Close()
	total := 0
	for _, h := range ds.Ranges {
		req := protocol.Request{Prefix: h.From, Opt: h.To}
		stream, err := m.runPull(tcp, &req)
		if err != nil {
			core.AppLog.Warn().Msgf("recovery pull error from %s: %s", ds.Remote, err.Error())
			continue
		}
		for {
			data, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				core.AppLog.Warn().Msgf("recovery recv error from %s: %s", ds.Remote, err.Error())
				break
			}
			total += len(data.Data.List)
			m.set(data)
		}
	}
	core.AppLog.Info().Msgf("recovery from %s complete, %d rows", ds.Remote, total)
}
func (m *MemberHashRingListener) localNode(node core.Node) bool {
	return strings.HasPrefix(node.Name, m.LocalNode().Name)
}

// memberlist callbacks
func (m *MemberHashRingListener) NodeAdded(added core.Node) {
	core.AppLog.Warn().Msgf("add node %v", added)
	select {
	case m.nRequest <- NodeRequest{opt: NODE_ADDED, node: added}:
	default:
		core.AppLog.Warn().Msgf("something wrong!!! %d", len(m.nRequest))
	}
}
func (m *MemberHashRingListener) NodeRemoved(removed core.Node) {

	m.nRequest <- NodeRequest{opt: NODE_REMOVED, node: removed}
}

func (m *MemberHashRingListener) NodeUpdated(updated core.Node) {
	m.nRequest <- NodeRequest{opt: NODE_UPDATED, node: updated}
}
func (m *MemberHashRingListener) NodesMerged(nodes []core.Node) {

}
func (m *MemberHashRingListener) NodesConflicted(nodes []core.Node) {

}
func (m *MemberHashRingListener) NodeLived(node core.Node) {

}
func (m *MemberHashRingListener) NodePinged(node core.Node) {

}
