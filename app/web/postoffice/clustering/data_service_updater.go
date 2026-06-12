package clustering

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type RingUpdate struct {
	State int
	Nodes []core.Node
}

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
)

type ReceiverAsync struct {
	Rev  chan *protocol.Mail
	Q    chan string
	Subs map[string]core.Subscription
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

func (m *DataServiceProvider) balanceOnNodeAdded(added RingUpdate) {

	if m.backRing.nodeNum == 0 {
		m.backRing.nodes = append(m.backRing.nodes, added.Nodes...)
		slices.SortFunc(m.backRing.nodes, cmp)
		m.backRing.nodeNum++
		return
	}
	slices.SortFunc(added.Nodes, cmp)
	ringSync := core.RingSync{Ranges: make([]core.RingRange, 0)}
	for _, n := range added.Nodes {
		if !m.Mll.localNode(n) { //skip node initial add call
			ringRange := m.backRing.rangeOfRing(n.RingToken)
			if m.Mll.localNode(ringRange[1]) {
				ringSync.Remote = ringRange[1].RpcEndpoint
				ringSync.Ranges = append(ringSync.Ranges, core.RingRange{From: ringRange[0].RingToken, To: n.RingToken})
				core.AppLog.Debug().Msgf("push data key hash >= %d and < %d to remote node %s", ringRange[0].RingToken, n.RingToken, n.IP)
			}
		}
		m.backRing.nodes = append(m.backRing.nodes, n)
		slices.SortFunc(m.backRing.nodes, cmp)
	}
	m.backRing.nodeNum++
	if len(ringSync.Ranges) == 0 {
		return
	}
	m.Mll.MRequest <- core.RingRequest{Source: ringSync, Opt: SYNC_NODE_OPT, Address: added.Nodes[0].IP}
	m.subscriptions.lookup(func(sub core.Subscription) {
		if sub.Endpoint == m.rpcEndpoint {
			m.Mll.MRequest <- core.RingRequest{Opt: SYNC_SUB_OPT, Address: added.Nodes[0].IP, Source: core.RingSync{Sub: sub}}
		}
	})
}

func (m *DataServiceProvider) balanceOnNodeRemoved(removed RingUpdate) {

	for _, n := range removed.Nodes {
		m.backRing.nodes = slices.DeleteFunc(m.backRing.nodes, func(d core.Node) bool {
			return d.IP == n.IP
		})
	}
	slices.SortFunc(m.backRing.nodes, cmp)
	m.backRing.nodeNum--
}

func (m *DataServiceProvider) registerSubscription(sub core.Subscription) {
	if sub.Type == core.TRANS_MAIL && !strings.HasPrefix(sub.Topic, TRANS_SUB_PREFIX) {
		sub.Topic = fmt.Sprintf("%s%s", TRANS_SUB_PREFIX, sub.Topic)
	}
	if sub.Deleting {
		m.subscriptions.del(sub)
		listener, ok := m.listeners[sub.NodeId]
		if !ok {
			return
		}
		delete(listener.Subs, sub.Topic)
		return
	}
	listener, ok := m.listeners[sub.NodeId]
	if !ok {
		listener = ReceiverAsync{Rev: make(chan *protocol.Mail, NODE_EVENT_BUFFER_SIZE), Q: make(chan string, 2), Subs: make(map[string]core.Subscription)}
		m.listeners[sub.NodeId] = listener
		m.listenerPool = append(m.listenerPool, sub.NodeId)
	}
	m.subscriptions.add(sub)
	listener.Subs[sub.Topic] = sub
}

func (m *DataServiceProvider) RingUpdated() {
	running := true
	subSync := time.NewTicker(60 * time.Second)
	defer subSync.Stop()
	for running {
		select {
		case <-subSync.C:
			m.subscriptions.lookup(func(sub core.Subscription) {
				if sub.Endpoint == m.rpcEndpoint {
					m.Mll.MRequest <- core.RingRequest{Opt: SYNC_SUB_OPT, Source: core.RingSync{Sub: sub}}
				}
			})
		case ringUpdate := <-m.RNode:
			switch ringUpdate.State {
			case NODE_STATE_SHUTDOWN:
				running = false
			case NODE_STATE_LIVE:
				m.balanceOnNodeAdded(ringUpdate)
			case NODE_STATE_DEAD:
				m.balanceOnNodeRemoved(ringUpdate)
			}

		case sync := <-m.RSync:
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
		case req := <-m.DRequest:
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
				sub := m.subscriptions.pick(name, req.Tag)
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
		case msg := <-m.DMessager:
			switch msg.Opt {
			case core.TOPIC_MAIL:
				for _, ch := range m.listeners {
					_, subed := ch.Subs[msg.Topic.Name]
					if subed {
						ch.Rev <- msg
					}
				}
			case core.TRANS_MAIL:
				tn := fmt.Sprintf("%s%s", TRANS_SUB_PREFIX, msg.Transaction.Meta.Name)
				lk := ""
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
						nc.Rev <- msg
						m.listenerPool = append(m.listenerPool[1:], nk) //add to tail
						break
					}
					//mark last one to break loop if fullly iterated
					lk = nk
					m.listenerPool = append(m.listenerPool[1:], nk) //add to tail
				}
			}
		}

	}
	//shutdown server
	for range SET_OPERATOR_NUM {
		m.DSet <- SetData{Opt: core.SET_OPT_CLOSE}
	}
	close(m.DSet)
	close(m.DPull)
	m.server.Stop()
	m.Local.Close()
	core.AppLog.Info().Msg("local data service provider has stopped")
}

// recoverFromNode pulls ring-partition data from ds.Remote in the background.
// Runs in its own goroutine so DSet workers stay available for normal writes.
func (m *DataServiceProvider) recoverFromNode(ds core.RingSync) {
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
