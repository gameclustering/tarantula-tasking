package clustering

import (
	context "context"
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runAskFinish(t *protocol.Meta) (*protocol.Response, error) {
	rq := make(chan []core.Subscription, 3)
	defer close(rq)
	c.DRequest <- TopicRequest{Opt: TASK_REGISTER, Subs: rq, NodeId: t.NodeId, Tag: t.Tag, Name: t.Name}
	subs := <-rq
	for _, sub := range subs {
		conn, err := sub.CPool.Conn()
		if err != nil {
			core.AppLog.Warn().Msgf("no connection available on sub %v", sub)
			continue
		}
		dsp := protocol.NewTransactionServiceClient(conn.Conn)
		return dsp.AskFinish(context.Background(), t)
	}
	core.AppLog.Warn().Msgf("no subscrition available %v", t)
	return &protocol.Response{Successful: false}, fmt.Errorf("no subscription available")
}
