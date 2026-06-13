package clustering

import (
	context "context"
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runAskFinish(t *protocol.Meta) (*protocol.Response, error) {
	core.AppLog.Info().Msgf("runAskFinish called txn=%d name=%s state=%d", t.Id, t.Name, t.State)
	rq := make(chan []core.Subscription, 3)
	defer close(rq)
	c.DRequest <- TopicRequest{Opt: TASK_ASSIGN, Subs: rq, Tag: t.Tag, Name: t.Name}
	subs := <-rq
	for _, sub := range subs {
		conn, err := sub.CPool.Conn()
		if err != nil {
			core.AppLog.Warn().Msgf("no connection available on sub %v", sub)
			continue
		}
		core.AppLog.Info().Msgf("runAskFinish dispatching txn=%d name=%s to=%s", t.Id, t.Name, sub.Endpoint)
		dsp := protocol.NewTransactionServiceClient(conn.Conn)
		return dsp.AskFinish(context.Background(), t)
	}
	core.AppLog.Warn().Msgf("no subscription available for finish %v", t)
	return &protocol.Response{Successful: false}, fmt.Errorf("no subscription available")
}
