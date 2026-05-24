package clustering

import (
	context "context"
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runConfirmed(t *protocol.Meta) (*protocol.Response, error) {
	if t.Prefix == 0 {
		core.AppLog.Error().Msgf("prefix not assigned %d", t.TaskId)
		return &protocol.Response{Successful: false}, fmt.Errorf("prefix must be assigned")
	}
	rq := make(chan []core.Node, 3)
	defer close(rq)
	c.Mll.MRequest <- core.RingRequest{Opt: REPLICA_RING_OPT, Token: t.Prefix, Replicas: REPLICA_MAX, Async: rq}
	nodes := <-rq
	ringNode := nodes[0]
	conn, err := ringNode.CPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	}
	dsp := protocol.NewTransactionServiceClient(conn.Conn)
	return dsp.Confirmed(context.Background(), t)
}
