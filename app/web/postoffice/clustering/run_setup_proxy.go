package clustering

import (
	context "context"
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runSetup(t *protocol.Task) (*protocol.Response, error) {
	if t.Meta.Prefix == 0 {
		core.AppLog.Error().Msgf("prefix not assigned %d", t.Meta.TaskId)
		return &protocol.Response{Successful: false}, fmt.Errorf("prefix must be assigned")
	}
	rq := make(chan []core.Node, 3)
	defer close(rq)
	c.Mll.MRequest <- core.RingRequest{Opt: REPLICA_RING_OPT, Token: t.Meta.Prefix, Replicas: REPLICA_MAX, Async: rq}
	nodes := <-rq
	ringNode := nodes[0]
	conn, err := ringNode.CPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	}
	dsp := protocol.NewTransactionServiceClient(conn.Conn)
	return dsp.Setup(context.Background(), t)
}
