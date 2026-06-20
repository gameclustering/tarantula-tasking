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
	core.AppLog.Info().Msgf("runAskReserve called txn=%d name=%s tag=%s", t.Meta.Id, t.Meta.Name, t.Meta.Tag)
	for attempt := 0; attempt < reserveRetryMax && c.running; attempt++ {
		rq := make(chan []core.Subscription, 3)
		c.DRequest <- TopicRequest{Opt: TASK_ASSIGN, Subs: rq, Tag: t.Meta.Tag, Name: t.Meta.Name}
		subs := <-rq
		close(rq)
		dispatched := false
		for _, sub := range subs {
			conn, err := sub.CPool.Conn()
			if err != nil {
				core.AppLog.Warn().Msgf("no connection available on sub %v", sub)
				continue
			}
			core.AppLog.Info().Msgf("runAskReserve dispatching txn=%d name=%s to=%s", t.Meta.Id, t.Meta.Name, sub.Endpoint)
			dsp := protocol.NewTransactionServiceClient(conn.Conn)
			ctx, cancel := context.WithTimeout(context.Background(), CLIENT_TIMEOUT)
			resp, err := dsp.AskReserve(ctx, t)
			cancel()
			if err != nil {
				core.AppLog.Warn().Msgf("runAskReserve AskReserve failed txn=%d endpoint=%s err=%s, retrying next subscriber", t.Meta.Id, sub.Endpoint, err.Error())
				dispatched = true
				break // try next round-robin subscriber immediately, without sleeping
			}
			return resp, err
		}
		if !dispatched && attempt+1 < reserveRetryMax {
			core.AppLog.Warn().Msgf("no subscription available for reserve %v, retry %d/%d in %s", t.Meta, attempt+1, reserveRetryMax, reserveRetryInterval)
			time.Sleep(reserveRetryInterval)
		}
	}
	core.AppLog.Warn().Msgf("no subscription available for reserve after %d attempts %v", reserveRetryMax, t.Meta)
	return &protocol.Response{Successful: false}, fmt.Errorf("no subscription available")
}
