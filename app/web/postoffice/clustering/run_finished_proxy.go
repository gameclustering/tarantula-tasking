package clustering

import (
	"context"
	"fmt"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runFinished(t *protocol.Meta) (*protocol.Response, error) {
	if t.Prefix == 0 {
		core.AppLog.Error().Msgf("prefix not assigned %d", t.TaskId)
		return &protocol.Response{Successful: false}, fmt.Errorf("prefix must be assigned")
	}
	rq := make(chan []core.Node, 3)
	defer close(rq)
	c.MRequest <- core.RingRequest{Opt: REPLICA_RING_OPT, Token: t.Prefix, Replicas: REPLICA_MAX, Async: rq}
	nodes := <-rq
	ringNode := nodes[0]
	if ringNode.RpcEndpoint == c.rpcEndpoint {
		t.State = protocol.TCC_FINISHED
		c.TManager.Update(t)
		return &protocol.Response{Successful: true}, nil
	}
	conn, err := ringNode.CPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	}
	dsp := protocol.NewTransactionServiceClient(conn.Conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return dsp.Finished(ctx, t)
}
