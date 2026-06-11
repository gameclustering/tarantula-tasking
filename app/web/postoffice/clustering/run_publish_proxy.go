package clustering

import (
	"context"
	"fmt"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runPublish(topic *protocol.Topic) (*protocol.Response, error) {
	tpf, registered := core.QueryFactoryRegistry[topic.Name]
	if !registered {
		return &protocol.Response{Successful: false}, fmt.Errorf("event factory not registered")
	}
	to := tpf()
	tp, ok := to.(core.ProtoTopicFactory)
	if !ok {
		return &protocol.Response{Successful: false}, fmt.Errorf("event factory cannot casted from %s", topic.Name)
	}
	topic.Event.Key.Header.Timestamp = uint64(time.Now().UnixMilli())
	req, err := tp.Request(topic)
	req.Prefix = tp.Hash(c.Mll)
	if err != nil {
		return &protocol.Response{Successful: false}, err
	}
	resp, err := c.runCreate(req)
	if !resp.Successful {
		core.AppLog.Warn().Msgf("cannot save topic %v", resp)
		return resp, fmt.Errorf("cannot save topic")
	}
	rq := make(chan []core.Subscription, 3)
	defer close(rq)
	c.DRequest <- TopicRequest{Opt: TOPIC_REGISTER, Subs: rq, NodeId: topic.NodeId, Tag: topic.Tag, Name: topic.Name}
	subs := <-rq
	// deliver to subscribers in parallel — each Send is fire-and-forget on the receiver side
	for _, sub := range subs {
		s := sub
		go func() {
			if _, err := c.clientPublish(&s, topic); err != nil {
				core.AppLog.Debug().Msgf("publish delivery error %s", err.Error())
			}
		}()
	}
	return &protocol.Response{Successful: true, Message: "topic delivered"}, nil
}

func (m *DataServiceProvider) clientPublish(target *core.Subscription, request *protocol.Topic) (*protocol.Response, error) {
	conn, err := target.CPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), CLIENT_TIMEOUT)
	defer cancel()
	dsp := protocol.NewDataServiceClient(conn.Conn)
	return dsp.Send(ctx, request)
}
