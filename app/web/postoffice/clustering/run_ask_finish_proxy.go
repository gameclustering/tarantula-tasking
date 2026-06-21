package clustering

import (
	context "context"
	"fmt"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runAskFinish(t *protocol.Meta) (*protocol.Response, error) {
	core.AppLog.Info().Msgf("runAskFinish called txn=%d name=%s state=%d nodeId=%s", t.Id, t.Name, t.State, t.NodeId)
	rq := make(chan []core.Subscription, 3)
	defer close(rq)
	c.DRequest <- TopicRequest{Opt: TASK_ASSIGN, Subs: rq, Tag: t.Tag, Name: t.Name, NodeId: t.NodeId}
	subs := <-rq
	for _, sub := range subs {
		core.AppLog.Info().Msgf("runAskFinish dispatching txn=%d name=%s to=%s", t.Id, t.Name, sub.Endpoint)
		if sub.Endpoint == c.rpcEndpoint {
			return c.AskFinish(context.Background(), t)
		}
		conn, err := sub.CPool.Conn()
		if err != nil {
			core.AppLog.Warn().Msgf("no connection available on sub %v", sub)
			continue
		}
		dsp := protocol.NewTransactionServiceClient(conn.Conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		resp, err := dsp.AskFinish(ctx, t)
		if err != nil {
			core.AppLog.Warn().Msgf("runAskFinish gRPC failed txn=%d endpoint=%s err=%s", t.Id, sub.Endpoint, err.Error())
		}
		return resp, err
	}
	core.AppLog.Warn().Msgf("no subscription available for finish %v", t)
	return &protocol.Response{Successful: false}, fmt.Errorf("no subscription available")
}
