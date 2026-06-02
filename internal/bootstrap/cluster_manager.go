package bootstrap

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/event"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	RPC_CONNECT_RETRIES int = 3

	SUB_CHAN_SIZE   int = 3
	TOPIC_CHAN_SIZE int = 12

	OPT_SUB   int = 100
	OPT_UNSUB int = 200

	OPT_TRANS   int = 300
	OPT_UNTRANS int = 400
)

type Sub struct {
	opt                 int
	name                string
	listener            core.TopicListener
	transactionListener core.TransactionListener
}

type ClusterManager struct {
	App     *AppManager
	running bool

	subscriptions map[string]core.TopicListener
	transactions  map[string]core.TransactionListener
	cSub          chan Sub
	cInboundTopic chan *protocol.Topic
	cInboundTrans chan *protocol.Transaction
	cHost         string
	cPool         core.RpcConnPool
	cLock         *sync.Mutex
}

func (c *ClusterManager) HashRing(r core.RingRequest) (*protocol.Response, error) {
	if !c.running {
		return nil, fmt.Errorf("cluster not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return nil, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.HashRing(context.Background(), &protocol.Request{Prefix: 0})
}

func (c *ClusterManager) KeyRing(r core.RingRequest) (*protocol.Response, error) {
	if !c.running {
		return nil, fmt.Errorf("cluster not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return nil, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.KeyRing(context.Background(), &protocol.Request{Prefix: r.Token})
}

func (c *ClusterManager) RingToken(key []byte) uint32 {
	c.cLock.Lock()
	defer c.cLock.Unlock()
	return util.Hash32(key)
}

func (c *ClusterManager) Request(r *protocol.Request) (*protocol.Response, error) {
	if !c.running {
		return &protocol.Response{Successful: false}, fmt.Errorf("cluster not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return nil, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.Request(context.Background(), r)
}

func (c *ClusterManager) List(r core.Query) (grpc.ServerStreamingClient[protocol.Response], error) {
	if !c.running {
		return nil, fmt.Errorf("cluster not started")
	}
	mf, existed := core.QueryFactoryRegistry[r.QTopic()]
	if !existed {
		return nil, fmt.Errorf("topic factory not existed")
	}
	dt, err := mf().Export(r)
	if err != nil {
		return nil, err
	}
	q := protocol.Query{Id: r.QTopic(), Criteria: dt}
	req := protocol.Request{Prefix: r.Hash(c), Query: &q}
	conn, err := c.cPool.Conn()
	if err != nil {
		return nil, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.List(context.Background(), &req)
}

func (c *ClusterManager) Publish(e *protocol.Topic) (*protocol.Response, error) {
	if !c.running {
		return &protocol.Response{Successful: false}, fmt.Errorf("not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false}, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.Publish(context.Background(), e)
}

func (c *ClusterManager) Issue(e *protocol.Task) (*protocol.Response, error) {
	if !c.running {
		return &protocol.Response{Successful: false}, fmt.Errorf("not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false}, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	e.Meta.Time = timestamppb.Now()
	return dsp.Issue(context.Background(), e)
}

func (c *ClusterManager) Confirm(e *protocol.Meta) (*protocol.Response, error) {
	if !c.running {
		return &protocol.Response{Successful: false}, fmt.Errorf("not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false}, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.Confirm(context.Background(), e)
}

func (c *ClusterManager) Cancel(e *protocol.Meta) (*protocol.Response, error) {
	if !c.running {
		return &protocol.Response{Successful: false}, fmt.Errorf("not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false}, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.Cancel(context.Background(), e)
}

func (c *ClusterManager) Finish(e *protocol.Meta) (*protocol.Response, error) {
	if !c.running {
		return &protocol.Response{Successful: false}, fmt.Errorf("not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false}, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.Finish(context.Background(), e)
}

func (c *ClusterManager) Subscribe(topic string, listener core.TopicListener) error {
	resp, err := c.subscribe(topic, core.TOPIC_MAIL)
	if err != nil {
		return err
	}
	c.cSub <- Sub{opt: OPT_SUB, name: topic, listener: listener}
	core.AppLog.Debug().Msgf("topic registered %v %s", resp.Successful, topic)
	return nil
}

func (c *ClusterManager) subscribe(name string, opt uint32) (*protocol.Response, error) {
	if !c.running {
		return &protocol.Response{Successful: false}, fmt.Errorf("not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false}, err
	}

	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.Subscribe(context.Background(), &protocol.Subscription{Opt: opt, NodeId: c.App.NodeId(), Tag: c.App.Context(), Name: name})
}
func (c *ClusterManager) unsubscribe(name string, opt uint32) (*protocol.Response, error) {
	if !c.running {
		return &protocol.Response{Successful: false}, fmt.Errorf("not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false}, err
	}

	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.Unsubscribe(context.Background(), &protocol.Subscription{Opt: opt, NodeId: c.App.NodeId(), Tag: c.App.Context(), Name: name})
}

func (c *ClusterManager) Unsubscribe(topic string) error {
	resp, err := c.unsubscribe(topic, core.TOPIC_MAIL)
	if err != nil {
		return err
	}
	c.cSub <- Sub{opt: OPT_UNSUB, name: topic}
	core.AppLog.Debug().Msgf("topic unregistered %v %s", resp.Successful, topic)
	return nil

}

func (c *ClusterManager) Register(name string, listener core.TransactionListener) error {
	resp, err := c.subscribe(name, core.TRANS_MAIL)
	if err != nil {
		return err
	}
	c.cSub <- Sub{opt: OPT_TRANS, name: name, transactionListener: listener}
	core.AppLog.Debug().Msgf("transaction registered %v %s", resp.Successful, name)
	return nil
}
func (c *ClusterManager) Unregister(name string) error {
	resp, err := c.unsubscribe(name, core.TRANS_MAIL)
	if err != nil {
		return err
	}
	c.cSub <- Sub{opt: OPT_UNTRANS, name: name}
	core.AppLog.Debug().Msgf("transaction unregistered %v %s", resp.Successful, name)
	return nil
}

func (c *ClusterManager) disconnect() error {
	if !c.running {
		return fmt.Errorf("not started")
	}
	c.running = false
	c.cPool.Release()
	return nil
}

func (c *ClusterManager) connect(host string) error {
	c.cHost = fmt.Sprintf("%s:%d", host, core.RPC_PORT)
	c.cPool = core.RpcConnPool{Target: c.cHost, Tag: c.App.Context(), NodeId: c.App.NodeId(), Auth: c.App.Auth, CACert: c.App.CACert}
	c.cPool.Start()
	c.subscriptions = make(map[string]core.TopicListener)
	c.transactions = make(map[string]core.TransactionListener)
	c.cSub = make(chan Sub, SUB_CHAN_SIZE)
	c.cInboundTopic = make(chan *protocol.Topic, TOPIC_CHAN_SIZE)
	c.cInboundTrans = make(chan *protocol.Transaction, TOPIC_CHAN_SIZE)
	c.cLock = &sync.Mutex{}
	c.running = true
	var wt sync.WaitGroup
	wt.Add(1)
	go c.async()
	go c.receive(&wt)
	wt.Wait()
	return nil
}

func (c *ClusterManager) receive(w *sync.WaitGroup) {
	retries := RPC_CONNECT_RETRIES
	conn, err := c.cPool.Conn()
	if err != nil {
		panic(err.Error())
	}
ro:
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	stream, err := dsp.Receive(context.Background(), &protocol.Topic{NodeId: c.App.NodeId(), Tag: c.App.Context()})
	if err != nil {
		retries--
		if retries > 0 {
			core.AppLog.Warn().Msgf("rpc connection retry with %s %d", err.Error(), retries)
			time.Sleep(3 * time.Second)
			goto ro
		}
		core.AppLog.Warn().Msgf("rpc connection error after retried %s", err.Error())
		return
	}
	w.Done()
	for c.running {
		resp, err := stream.Recv()
		if err == io.EOF {
			core.AppLog.Warn().Msgf("eof %s", err.Error())
			break
		}
		if err != nil {
			core.AppLog.Warn().Msgf("streaming error %s", err.Error())
			break
		}
		switch resp.Opt {
		case core.TOPIC_MAIL:
			c.cInboundTopic <- resp.Topic
		case core.TRANS_MAIL:
			c.cInboundTrans <- resp.Transaction
		}
	}
	core.AppLog.Warn().Msgf("cluster manager receiver closed from remote %v", c.running)
}

func (c *ClusterManager) async() {
	for c.running {
		select {
		case tran := <-c.cInboundTrans:
			tl, ok := c.transactions[tran.Meta.Name]
			if ok {
				go c.handleTranstion(tl, tran)
			} else {
				core.AppLog.Warn().Msgf("dead task %v", tran)
			}
		case topic := <-c.cInboundTopic:
			tl, ok := c.subscriptions[topic.Name]
			if ok {
				tl.OnTopic(topic)
			} else {
				core.AppLog.Warn().Msgf("dead topic %v", topic)
			}
		case sub := <-c.cSub:
			switch sub.opt {
			case OPT_SUB:
				c.subscriptions[sub.name] = sub.listener
			case OPT_UNSUB:
				delete(c.subscriptions, sub.name)
			case OPT_TRANS:
				c.transactions[sub.name] = sub.transactionListener
			case OPT_UNTRANS:
				delete(c.transactions, sub.name)
			}
		}
	}
	core.AppLog.Warn().Msgf("cluster manager async task closed from remote %v", c.running)
	clear(c.subscriptions)
	close(c.cInboundTopic)
	close(c.cInboundTrans)
	close(c.cSub)
}

func (c *ClusterManager) Forward(level zerolog.Level, log []byte) {
	if !c.running {
		return
	}
	e := protocol.LogEvent{}
	err := protojson.Unmarshal(log, &e)
	if err != nil {
		e.Level = "error"
		e.Message = err.Error()
		e.Time = timestamppb.Now()
		e.Source = "forwarder:359"
	}
	tf := event.LogEventFactory{}
	t, err := tf.FromLogEvent(&e)
	t.NodeId = c.App.NodeId()
	t.Tag = c.App.Context()
	t.Event.Key.Array = core.ToBytes(c.App.seq)
	go c.Publish(t)
}

func (c *ClusterManager) handleTranstion(l core.TransactionListener, t *protocol.Transaction) {
	state := t.Meta.State
	t.Meta.NodeId = c.App.NodeId()
	t.Meta.Tag = c.App.Context()
	err := l.OnTransaction(t)
	if err != nil {
		//cancel
		//send cancel to cluster
		t.Meta.State = protocol.TCC_CANCELED
		t.Meta.Description = err.Error()
		c.Cancel(t.Meta)
		return
	}
	switch state {
	case protocol.TCC_RESERVING:
		t.Meta.State = protocol.TCC_CONFIRMED
		//send confirmed to cluster
		c.Confirm(t.Meta)

	case protocol.TCC_CONFIRMED:
		//send commited to cluster
		t.Meta.State = protocol.TCC_FINISHED
		c.Finish(t.Meta)
	case protocol.TCC_CANCELED:
		//send aborted to cluster
		t.Meta.State = protocol.TCC_FINISHED
		c.Finish(t.Meta)
	}

}

func (c *ClusterManager) TopicList() (*protocol.Response, error) {
	if !c.running {
		return nil, fmt.Errorf("cluster not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return nil, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.TopicList(context.Background(), &protocol.Request{Prefix: 0})
}

func (c *ClusterManager) TaskList() (*protocol.Response, error) {
	if !c.running {
		return nil, fmt.Errorf("cluster not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return nil, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.TaskList(context.Background(), &protocol.Request{Prefix: 0})
}

func (c *ClusterManager) AuthKey(name string) (*protocol.AuthKey, error) {
	if !c.running {
		return nil, fmt.Errorf("cluster not started")
	}
	conn, err := c.cPool.Conn()
	if err != nil {
		return nil, err
	}
	dsp := protocol.NewPostofficeServiceClient(conn.Conn)
	return dsp.AuthKey(context.Background(), &protocol.Request{Context: fmt.Sprintf("%s#%s", c.App.F.PresenceCtx(), name)})
}
