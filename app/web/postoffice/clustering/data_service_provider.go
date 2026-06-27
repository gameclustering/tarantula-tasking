package clustering

import (
	context "context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"github.com/hashicorp/memberlist"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

const (
	PULL_BATCH_SIZE int = 100
)

type DataServiceProvider struct {
	protocol.UnimplementedDataServiceServer
	protocol.UnimplementedPostofficeServiceServer
	protocol.UnimplementedTransactionServiceServer
	Local *persistence.BadgerLocal

	server *grpc.Server

	backRing    NodeRing
	rpcEndpoint string
	TLSCert     tls.Certificate // leaf cert generated at startup for the gRPC server
	CACert      []byte          // PEM CA cert; passed to inter-node RpcConnPools for server verification
	seq         core.Sequence
	auth        core.Authenticator
	//write worker chan
	DWait    sync.WaitGroup
	running  bool
	shutdown chan struct{}
	*MemberHashRing
	//topic message
	DMessager     chan *protocol.Mail
	subscriptions SubscriptionRegistry

	listeners    map[string]ReceiverAsync //chan *protocol.Topic
	listenerPool []string                 //roundrobin pool
	DRequest     chan TopicRequest
	MRequest     chan core.RingRequest //ring request
	nRequest     chan NodeRequest      //node event
	sRquest      chan RegisterRequest  //sub event

	//task transaction
	TManager *TaskManager
	vault    *util.VaultClient
	*memberlist.Memberlist
}

func (c *DataServiceProvider) Get(ctx context.Context, in *protocol.Request) (*protocol.Response, error) {
	getdata := GetData{in}
	data, err := c.get(getdata)
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	}
	return &protocol.Response{Successful: true, Data: &protocol.DataSet{List: []*protocol.Data{data}}}, nil

}

func (c *DataServiceProvider) Query(request *protocol.Request, stream grpc.ServerStreamingServer[protocol.Response]) error {

	tf, existed := core.QueryFactoryRegistry[request.Query.Id]
	if !existed {
		return fmt.Errorf("event factory not registered %s", request.Query.Id)
	}
	q, err := tf().Import(request.Query.Criteria)
	if err != nil {
		return err
	}
	return c.query(q, stream)
}

func (c *DataServiceProvider) Reset(ctx context.Context, in *protocol.Request) (*protocol.Response, error) {
	sd := SetData{Opt: in.Opt, Prefix: in.Prefix, Data: in.Data}
	if sd.Prefix == 0 {
		sd.Prefix = c.RingToken(sd.Key)
	}
	ki, err := c.reset(sd)
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()},err
	} else {
		var data []*protocol.Data
		data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
		return &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}},nil
	}
}

func (c *DataServiceProvider) Create(ctx context.Context, in *protocol.Request) (*protocol.Response, error) {
	sd := SetData{Opt: in.Opt, Prefix: in.Prefix, Data: in.Data}
	if sd.Prefix == 0 {
		sd.Prefix = c.RingToken(sd.Key)
	}
	ki, err := c.create(sd)
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	} else {
		var data []*protocol.Data
		data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
		return &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}, nil
	}
}

func (c *DataServiceProvider) Update(ctx context.Context, in *protocol.Request) (*protocol.Response, error) {
	sd := SetData{Opt: in.Opt, Prefix: in.Prefix, Data: in.Data}
	if sd.Prefix == 0 {
		c.RingToken(sd.Key)
	}
	ki, err := c.update(sd)
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	} else {
		var data []*protocol.Data
		data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
		return &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}, nil
	}
}

func (c *DataServiceProvider) Delete(ctx context.Context, in *protocol.Request) (*protocol.Response, error) {
	sd := SetData{Opt: in.Opt, Prefix: in.Prefix, Data: in.Data}
	if sd.Prefix == 0 {
		c.RingToken(sd.Key)
	}
	ki, err := c.delete(sd)
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, nil
	} else {
		var data []*protocol.Data
		data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
		return &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}, nil
	}

}

func (c *DataServiceProvider) SyncRingRange(request *protocol.RingRange, stream grpc.ServerStreamingServer[protocol.Response]) error {
	return c.pull(request.From, request.To, stream)
}

func (c *DataServiceProvider) Send(ctx context.Context, in *protocol.Topic) (*protocol.Response, error) {
	select {
	case c.DMessager <- &protocol.Mail{Topic: in, Opt: core.TOPIC_MAIL}:
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	return &protocol.Response{Successful: true, Message: "event published"}, nil
}

func (c *DataServiceProvider) Register(ctx context.Context, in *protocol.Subscription) (*protocol.Response, error) {
	sub := core.Subscription{}
	sub.FromProto(in)
	sub.Deleting = false
	select {
	case c.sRquest <- RegisterRequest{sub: sub}:
		return &protocol.Response{Successful: true}, nil
	default:
		core.AppLog.Warn().Msgf("oops channel is full %d", len(c.sRquest))
		return &protocol.Response{Successful: false}, fmt.Errorf("channel full")
	}
}

func (c *DataServiceProvider) Unregister(ctx context.Context, in *protocol.Subscription) (*protocol.Response, error) {
	sub := core.Subscription{}
	sub.FromProto(in)
	sub.Deleting = true
	select {
	case c.sRquest <- RegisterRequest{sub: sub}:
		return &protocol.Response{Successful: true}, nil
	default:
		core.AppLog.Warn().Msgf("oops channel is full %d", len(c.sRquest))
		return &protocol.Response{Successful: false}, fmt.Errorf("channel full")
	}
}
func (c *DataServiceProvider) SyncSubs(ctx context.Context, in *protocol.SubsSync) (*protocol.Response, error) {
	for _, p := range in.Subs {
		sub := core.Subscription{}
		sub.FromProto(p)
		sub.Deleting = false
		select {
		case c.sRquest <- RegisterRequest{sub: sub}:
		default:
			core.AppLog.Warn().Msgf("oops channel is full %d", len(c.sRquest))
		}
	}
	return &protocol.Response{Successful: true}, nil
}

func (c *DataServiceProvider) NotifyRingSync(ctx context.Context, in *protocol.RingSync) (*protocol.Response, error) {
	go c.recoverFromNode(in)
	return &protocol.Response{Successful: true}, nil
}

func (c *DataServiceProvider) Start(dir string, ctx string) {
	c.running = true
	c.shutdown = make(chan struct{})
	c.backRing = NodeRing{nodes: make([]core.Node, 0)}
	path := fmt.Sprintf("%s/%s", dir, "store")
	core.AppLog.Warn().Msgf("creating path %s if not existed", path)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(err)
	}
	creds := credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{c.TLSCert}})
	c.Local = &persistence.BadgerLocal{Path: path, InMemory: false, LogDisabled: false, GcEnabled: true}
	err = c.Local.Open()
	if err != nil {
		panic(err)
	}
	c.DMessager = make(chan *protocol.Mail, NODE_EVENT_BUFFER_SIZE)
	c.DRequest = make(chan TopicRequest, NODE_EVENT_BUFFER_SIZE)
	c.MRequest = make(chan core.RingRequest, NODE_EVENT_BUFFER_SIZE)
	c.nRequest = make(chan NodeRequest, NODE_EVENT_BUFFER_SIZE)
	c.sRquest = make(chan RegisterRequest, NODE_EVENT_BUFFER_SIZE)

	c.listeners = make(map[string]ReceiverAsync) //chan *protocol.Topic)
	c.listenerPool = make([]string, 0)
	c.subscriptions = SubscriptionRegistry{topicEnds: make(map[core.TopicKey]map[string]core.Subscription), cPools: make(map[core.TopicKey]*core.RpcConnPool), roundIdx: make(map[string]int), auth: c.auth, caCert: c.CACert}

	c.TManager = &TaskManager{
		trs:        make(map[uint64]*TaskResource),
		tjs:        make(map[uint64]*JobResource),
		tms:        make(map[uint64]*Timeout),
		s:          c,
		tasks:      make(chan *protocol.Task, 10),
		updates:    make(chan *protocol.Meta, 10),
		jobs:       make(chan *protocol.Job, 10),
		recoveries: make(chan uint64, 10),
	}
	go c.TManager.Wait()
	tcp, err := net.Listen("tcp", fmt.Sprintf(":%d", core.RPC_PORT))
	if err != nil {
		panic(err)
	}
	ep := keepalive.EnforcementPolicy{MinTime: 30 * time.Second, PermitWithoutStream: true}
	rpc := grpc.NewServer(grpc.Creds(creds), grpc.KeepaliveEnforcementPolicy(ep), grpc.UnaryInterceptor(c.auditCall), grpc.StreamInterceptor(c.auditStreaming))
	c.server = rpc
	protocol.RegisterDataServiceServer(rpc, c)
	protocol.RegisterPostofficeServiceServer(rpc, c)
	protocol.RegisterTransactionServiceServer(rpc, c)
	core.AppLog.Debug().Msgf("local data service provider started on : %s", tcp.Addr().String())
	c.DWait.Done()
	err = rpc.Serve(tcp)
	if err != nil {
		panic(err)
	}
}

func (c *DataServiceProvider) recoverFromNode(ds *protocol.RingSync) {
	cpool := core.RpcConnPool{Target: ds.Remote, Auth: c.auth, CACert: c.caCert}
	cpool.Start()
	conn, err := cpool.Conn()
	if err != nil {
		core.AppLog.Warn().Msgf("remote rpcs connection error from %s: %s", ds.Remote, err.Error())
		return
	}
	defer cpool.Shutdown()

	total := 0
	for _, h := range ds.Ranges {
		subtotal, err := c.runSyncRingRange(conn.Conn, h)
		if err != nil {
			core.AppLog.Warn().Msgf("recovery pull error from %s: %s", ds.Remote, err.Error())
		}
		total += subtotal
	}
	core.AppLog.Info().Msgf("recovery from %s complete, %d rows", ds.Remote, total)
}

func (c *DataServiceProvider) runSyncRingRange(conn *grpc.ClientConn, in *protocol.RingRange) (int, error) {
	subtotal := 0
	dsp := protocol.NewDataServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	stream, err := dsp.SyncRingRange(ctx, in)
	if err != nil {
		return 0, err
	}
	for {
		data, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return subtotal, err
		}
		subtotal += len(data.Data.List)
		c.set(data)
	}
	return subtotal, nil
}

func (c *DataServiceProvider) shuwdownHook() {
	core.AppLog.Warn().Msg("running data service provider shutdown hook ...")
	close(c.nRequest)
	close(c.sRquest)
	close(c.MRequest)
	close(c.DMessager)
	close(c.DRequest)
}
