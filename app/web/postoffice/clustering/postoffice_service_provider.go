package clustering

import (
	context "context"
	"fmt"
	"strings"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func (c *DataServiceProvider) AuthKey(ctx context.Context, request *protocol.Request) (*protocol.AuthKey, error) {
	core.AppLog.Info().Msgf("load auth key %s", request.Context)
	mp := strings.Split(request.Context, "#")
	if len(mp) != 2 {
		core.AppLog.Fatal().Msgf("wrong context format %s", request.Context)
		return &protocol.AuthKey{Context: request.Context}, fmt.Errorf("wrong context format %s", request.Context)
	}
	return c.vault.Load(mp[0], mp[1])
}

func (c *DataServiceProvider) HashRing(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	rq := make(chan []core.Node, 1)
	defer close(rq)
	c.MRequest <- (core.RingRequest{Async: rq, Opt: ALL_RING_OPT})
	select {
	case ring := <-rq:
		nodes := make([]*protocol.HashNode, 0)
		for _, n := range ring {
			hn := protocol.HashNode{Hash: n.RingToken, Endpoint: n.RpcEndpoint, Name: n.Name, Address: n.IP, Meta: n.Meta}
			nodes = append(nodes, &hn)
		}
		return &protocol.Response{Nodes: nodes}, nil
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unkown error")
	}
}

func (c *DataServiceProvider) KeyRing(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	rq := make(chan []core.Node, 1)
	defer close(rq)
	c.MRequest <- (core.RingRequest{Async: rq, Opt: REPLICA_RING_OPT, Token: request.Prefix})
	ring := <-rq
	nodes := make([]*protocol.HashNode, 0)
	for _, n := range ring {
		hn := protocol.HashNode{Hash: n.RingToken, Endpoint: n.RpcEndpoint, Name: n.Name, Address: n.IP}
		nodes = append(nodes, &hn)
	}
	return &protocol.Response{Nodes: nodes}, nil
}

func (c *DataServiceProvider) Receive(topic *protocol.Topic, stream grpc.ServerStreamingServer[protocol.Mail]) error {
	ctx := stream.Context()
	aq := make(chan ReceiverAsync, 2)
	select {
	case c.DRequest <- TopicRequest{Opt: RECEIVER_START, Name: topic.NodeId, Async: aq}:
	case <-ctx.Done():
		return ctx.Err()
	}
	var ch ReceiverAsync
	select {
	case ch = <-aq:
	case <-ctx.Done():
		return ctx.Err()
	}
	close(aq)
	core.AppLog.Info().Msgf("start event receiver on [%s]", topic.NodeId)
	receiving := true
	for receiving {
		select {
		case <-ch.Q:
			receiving = false
		case resp := <-ch.Rev:
			err := stream.Send(resp)
			if err != nil {
				core.AppLog.Debug().Msgf("send error %s", err.Error())
				receiving = false
			}
		case <-ctx.Done():
			receiving = false
		}
	}
	ctx5, cancel5 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5()
	select {
	case c.DRequest <- TopicRequest{Opt: RECEIVER_REMOVE, Name: topic.NodeId, QChan: ch.Q}:
	case <-ctx5.Done():
	}
	core.AppLog.Debug().Msgf("stop evnt receiver from on [%s]", topic.NodeId)
	ctx5b, cancel5b := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5b()
	select {
	case c.MRequest <- core.RingRequest{Opt: SYNC_SUB_OPT, Source: core.RingSync{Sub: core.Subscription{NodeId: topic.NodeId, Deleting: true}}}:
	case <-ctx5b.Done():
	}
	return nil
}
func (c *DataServiceProvider) Disconnect(ctx context.Context, topic *protocol.Topic) (*protocol.Response, error) {
	core.AppLog.Debug().Msgf("receiver disconnected %s", topic.NodeId)
	c.DRequest <- TopicRequest{Opt: RECEIVER_END, Name: topic.NodeId}
	return &protocol.Response{Successful: true, Message: "disconnected"}, nil
}
func (c *DataServiceProvider) Publish(ctx context.Context, in *protocol.Topic) (*protocol.Response, error) {
	return c.runPublish(in)
}

func (c *DataServiceProvider) Subscribe(ctx context.Context, in *protocol.Subscription) (*protocol.Response, error) {
	sub := core.Subscription{Type: in.Opt, NodeId: in.NodeId, Tag: in.Tag, Topic: in.Name, Endpoint: c.rpcEndpoint}
	// Register locally immediately via MSync — avoids relying on the memberlist self-loopback.
	//c.Mll.MSync <- util.ToJson(core.RingSync{Sub: sub})
	// Broadcast to all cluster members.
	c.MRequest <- core.RingRequest{Opt: SYNC_SUB_OPT, Source: core.RingSync{Sub: sub}}
	return &protocol.Response{Successful: true, Message: "topic created"}, nil
}
func (c *DataServiceProvider) Unsubscribe(ctx context.Context, in *protocol.Subscription) (*protocol.Response, error) {
	sub := core.Subscription{Type: in.Opt, NodeId: in.NodeId, Tag: in.Tag, Topic: in.Name, Endpoint: c.rpcEndpoint, Deleting: true}
	//c.Mll.MSync <- util.ToJson(core.RingSync{Sub: sub})
	c.MRequest <- core.RingRequest{Opt: SYNC_SUB_OPT, Source: core.RingSync{Sub: sub}}
	return &protocol.Response{Successful: true, Message: "topic removed"}, nil
}

func (c *DataServiceProvider) Request(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	switch request.Opt {

	case core.GET_DATA_REQUEST:
		return c.runGet(request)

	case core.CREATE_DATA_REQUEST:
		return c.runCreate(request)

	case core.UPDATE_DATA_REQUEST:
		return c.runUpdate(request)

	case core.DELETE_DATA_REQUEST:
		return c.runDelete(request)

	case core.RESET_DATA_REQUEST:
		return c.runReset(request)

	default:
	}
	return &protocol.Response{Successful: false, Message: "not suppotred"}, fmt.Errorf("opt not supported %d", request.Opt)
}

func (c *DataServiceProvider) List(in *protocol.Request, stream grpc.ServerStreamingServer[protocol.Response]) error {
	c.runQuery(in, stream)
	return nil
}

func (c *DataServiceProvider) Issue(ctx context.Context, task *protocol.Task) (*protocol.Response, error) {
	task.Meta.Id = c.tid()
	if task.Validator != nil {
		task.Validator.Meta.TaskId = task.Meta.Id
		task.Validator.Meta.Id = c.tid()
		task.Validator.Meta.Timeout = JOB_TIMEOUT_SECONDS
		for _, t := range task.Validator.Transactions {
			t.Meta.TaskId = task.Meta.Id
			t.Meta.JobId = task.Validator.Meta.Id
			t.Meta.Id = c.tid()
			t.Meta.Timeout = TRANSACTION_TIMEOUT_SECONDS
			t.Meta.Retries = TCC_RETRY_MAX
		}
	}
	jz := len(task.Jobs)
	if jz == 0 {
		return &protocol.Response{Successful: false, Message: ""}, fmt.Errorf("one more task jobs reqiured")
	}
	for _, job := range task.Jobs {
		job.Meta.TaskId = task.Meta.Id
		job.Meta.Id = c.tid()
		job.Meta.Timeout = JOB_TIMEOUT_SECONDS
		for _, t := range job.Transactions {
			t.Meta.TaskId = task.Meta.Id
			t.Meta.JobId = job.Meta.Id
			t.Meta.Id = c.tid()
			t.Meta.Timeout = TRANSACTION_TIMEOUT_SECONDS
			t.Meta.Retries = TCC_RETRY_MAX
		}
	}
	tb := persistence.TaskBuilder{Target: task}
	req, err := tb.HashRequest(c)
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	}
	resp, err := c.runCreate(req)
	if err != nil {
		return resp, err
	}
	if !resp.Successful {
		return resp, fmt.Errorf("task create failed: %s", resp.Message)
	}
	return c.runSetup(task)
}

func (c *DataServiceProvider) Confirm(ctx context.Context, meta *protocol.Meta) (*protocol.Response, error) {
	//call Confirmed
	c.runConfirmed(meta)
	return &protocol.Response{Successful: true}, nil
}

func (c *DataServiceProvider) Cancel(ctx context.Context, meta *protocol.Meta) (*protocol.Response, error) {
	//call Canceled
	c.runCanceled(meta)
	return &protocol.Response{Successful: true}, nil
}

func (c *DataServiceProvider) Finish(ctx context.Context, meta *protocol.Meta) (*protocol.Response, error) {
	resp, err := c.runFinished(meta)
	if err != nil {
		core.AppLog.Error().Msgf("runFinished failed txn=%d: %s", meta.Id, err.Error())
		return resp, err
	}
	return resp, nil
}

func (c *DataServiceProvider) TopicList(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
	rq := make(chan []core.Subscription, 3)
	defer close(rq)
	select {
	case c.DRequest <- TopicRequest{Opt: TOPIC_LIST, Subs: rq}:
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	var subs []core.Subscription
	select {
	case subs = <-rq:
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	tps := make([]*protocol.Subscription, 0)
	for _, sub := range subs {
		tps = append(tps, &protocol.Subscription{NodeId: sub.NodeId, Tag: sub.Tag, Name: sub.Topic, Endpoint: sub.Endpoint})
	}
	return &protocol.Response{Subscriptions: tps}, nil
}

func (c *DataServiceProvider) TaskList(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
	rq := make(chan []core.Subscription, 3)
	defer close(rq)
	select {
	case c.DRequest <- TopicRequest{Opt: TASK_LIST, Subs: rq}:
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	var subs []core.Subscription
	select {
	case subs = <-rq:
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	tks := make([]*protocol.Subscription, 0)
	for _, sub := range subs {
		tks = append(tks, &protocol.Subscription{NodeId: sub.NodeId, Tag: sub.Tag, Name: sub.Topic, Endpoint: sub.Endpoint})
	}
	return &protocol.Response{Subscriptions: tks}, nil
}

func (c *DataServiceProvider) tid() uint64 {
	return c.seq.UId()
}
