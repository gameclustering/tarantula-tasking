package clustering

import (
	context "context"
	"fmt"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

const (
	reserveRetryInterval = 30 * time.Second
	reserveRetryMax      = 10 // 10 × 30s = 5 min; exits well before the 1800s TManager retry
)

func (c *DataServiceProvider) runAskReserve(t *protocol.Transaction) (*protocol.Response, error) {
	for attempt := 0; attempt < reserveRetryMax && c.running; attempt++ {
		rq := make(chan []core.Subscription, 3)
		c.DRequest <- TopicRequest{Opt: TASK_ASSIGN, Subs: rq, Tag: t.Meta.Tag, Name: t.Meta.Name}
		subs := <-rq
		close(rq)
		for _, sub := range subs {
			conn, err := sub.CPool.Conn()
			if err != nil {
				core.AppLog.Warn().Msgf("no connection available on sub %v", sub)
				continue
			}
			dsp := protocol.NewTransactionServiceClient(conn.Conn)
			return dsp.AskReserve(context.Background(), t)
		}
		if attempt+1 < reserveRetryMax {
			core.AppLog.Warn().Msgf("no subscription available for reserve %v, retry %d/%d in %s", t.Meta, attempt+1, reserveRetryMax, reserveRetryInterval)
			time.Sleep(reserveRetryInterval)
		}
	}
	core.AppLog.Warn().Msgf("no subscription available for reserve after %d attempts %v", reserveRetryMax, t.Meta)
	return &protocol.Response{Successful: false}, fmt.Errorf("no subscription available")
}
