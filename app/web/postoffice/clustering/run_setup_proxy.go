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
	core.AppLog.Info().Msgf("runSetup task=%d prefix=%d ring_owner=%s local=%s is_local=%v", t.Meta.Id, t.Meta.Prefix, ringNode.RpcEndpoint, c.rpcEndpoint, ringNode.RpcEndpoint == c.rpcEndpoint)
	if ringNode.RpcEndpoint == c.rpcEndpoint {
		return c.Setup(context.Background(), t)
	}
	conn, err := ringNode.CPool.Conn()
	if err != nil {
		core.AppLog.Error().Msgf("runSetup CPool.Conn failed for %s: %s", ringNode.RpcEndpoint, err.Error())
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	}
	dsp := protocol.NewTransactionServiceClient(conn.Conn)
	ctx, cancel := context.WithTimeout(context.Background(), CLIENT_TIMEOUT)
	defer cancel()
	resp, err := dsp.Setup(ctx, t)
	core.AppLog.Info().Msgf("runSetup remote Setup resp=%v err=%v", resp, err)
	return resp, err
}
