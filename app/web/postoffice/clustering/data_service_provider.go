package clustering

import (
	context "context"
	"fmt"
	"net"
	"os"
	"sync"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	PULL_BATCH_SIZE int = 100

)

type DataServiceProvider struct {
	protocol.UnimplementedDataServiceServer
	protocol.UnimplementedPostofficeServiceServer
	protocol.UnimplementedTransactionServiceServer
	Local       *persistence.BadgerLocal
	RNode       <-chan RingUpdate
	RSync       <-chan []byte
	server      *grpc.Server
	Mll         *MemberListListener
	backRing    NodeRing
	rpcEndpoint string
	seq         core.Sequence
	auth        core.Authenticator
	//write worker chan
	DSet    chan SetData
	DPull   chan core.RingSync
	DWait   sync.WaitGroup
	running bool

	//topic message
	DMessager     chan *protocol.Mail
	subscriptions SubscriptionRegistry

	listeners    map[string]ReceiverAsync //chan *protocol.Topic
	listenerPool []string                 //roundrobin pool
	DRequest     chan TopicRequest

	//task transaction
	TManager *TaskManager
	vault    *util.VaultClient
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
	msg := make(chan *protocol.Response, 1)
	defer close(msg)
	setData := SetData{Opt: in.Opt, Prefix: in.Prefix, Data: in.Data, Resp: msg}
	c.DSet <- setData
	resp := <-msg
	return resp, nil
}

func (c *DataServiceProvider) Create(ctx context.Context, in *protocol.Request) (*protocol.Response, error) {
	msg := make(chan *protocol.Response, 1)
	defer close(msg)
	setData := SetData{Opt: in.Opt, Prefix: in.Prefix, Data: in.Data, Resp: msg}
	if setData.Prefix == 0 {
		setData.Prefix = c.Mll.RingToken(setData.Key)
	}
	c.DSet <- setData
	resp := <-msg
	return resp, nil
}

func (c *DataServiceProvider) Update(ctx context.Context, in *protocol.Request) (*protocol.Response, error) {
	msg := make(chan *protocol.Response, 1)
	defer close(msg)
	setData := SetData{Opt: in.Opt, Prefix: in.Prefix, Data: in.Data, Resp: msg}
	if setData.Prefix == 0 {
		c.Mll.RingToken(setData.Key)
	}
	c.DSet <- setData
	resp := <-msg
	return resp, nil
}

func (c *DataServiceProvider) Delete(ctx context.Context, in *protocol.Request) (*protocol.Response, error) {
	msg := make(chan *protocol.Response, 1)
	defer close(msg)
	setData := SetData{Opt: in.Opt, Prefix: in.Prefix, Data: in.Data, Resp: msg}
	if setData.Prefix == 0 {
		c.Mll.RingToken(setData.Key)
	}
	c.DSet <- setData
	resp := <-msg
	return resp, nil
}

func (c *DataServiceProvider) Pull(request *protocol.Request, stream grpc.ServerStreamingServer[protocol.Response]) error {
	return c.pull(request.Prefix, request.Opt, stream)
}

func (c *DataServiceProvider) Send(ctx context.Context, in *protocol.Topic) (*protocol.Response, error) {
	c.DMessager <- &protocol.Mail{Topic: in, Opt: core.TOPIC_MAIL}
	return &protocol.Response{Successful: true, Message: "event published"}, nil
}

func (c *DataServiceProvider) Start(dir string, ctx string) {
	c.running = true
	c.backRing = NodeRing{nodes: make([]core.Node, 0)}
	path := fmt.Sprintf("%s/%s", dir, "store")
	core.AppLog.Warn().Msgf("creating path %s if not existed", path)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(err)
	}
	creds, err := credentials.NewServerTLSFromFile(core.CERT_NAME, core.KEY_NAME)
	if err != nil {
		panic(err.Error())
	}
	c.Local = &persistence.BadgerLocal{Path: path, InMemory: false, LogDisabled: false, GcEnabled: true}
	err = c.Local.Open()
	if err != nil {
		panic(err)
	}
	c.DMessager = make(chan *protocol.Mail, NODE_EVENT_BUFFER_SIZE)
	c.DRequest = make(chan TopicRequest, NODE_EVENT_BUFFER_SIZE)
	c.listeners = make(map[string]ReceiverAsync) //chan *protocol.Topic)
	c.listenerPool = make([]string, 0)
	c.subscriptions = SubscriptionRegistry{topicEnds: make(map[core.TopicKey]map[string]core.Subscription), cPools: make(map[core.TopicKey]*core.RpcConnPool), auth: c.auth}

	c.DSet = make(chan SetData, NODE_EVENT_BUFFER_SIZE)
	c.DPull = make(chan core.RingSync, NODE_EVENT_BUFFER_SIZE)
	for n := range SET_OPERATOR_NUM {
		go c.runSetData(n)
	}
	c.TManager = &TaskManager{trs: make(map[uint64]*TaskResource), tjs: make(map[uint64]*JobResource), tms: make(map[uint64]*Timeout), s: c}
	go c.TManager.Wait()
	tcp, err := net.Listen("tcp", fmt.Sprintf(":%d", core.RPC_PORT))
	if err != nil {
		panic(err)
	}
	rpc := grpc.NewServer(grpc.Creds(creds), grpc.UnaryInterceptor(c.auditCall), grpc.StreamInterceptor(c.auditStreaming))
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
