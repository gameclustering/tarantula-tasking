package clustering

import (
	context "context"
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runAskReserve(t *protocol.Transaction) (*protocol.Response, error) {
	rq := make(chan []core.Subscription, 3)
	defer close(rq)
	c.DRequest <- TopicRequest{Opt: TASK_REGISTER, Subs: rq, NodeId: t.Meta.NodeId, Tag: t.Meta.Tag, Name: t.Meta.Name}
	subs := <-rq
	for _, sub := range subs {
		conn, err := sub.CPool.Conn()
		if err != nil {
			core.AppLog.Warn().Msgf("no connection available on sub %v", sub)
			continue
		}
		dsp := protocol.NewTransactionServiceClient(conn.Conn)
		return dsp.AskReserve(context.Background(), t)
	}
	core.AppLog.Warn().Msgf("no subscrition available %v", t.Meta)
	return &protocol.Response{Successful: false}, fmt.Errorf("no subscription available")
}
