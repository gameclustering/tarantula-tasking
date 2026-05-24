package clustering

import (
	context "context"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runGet(set *protocol.Request) (*protocol.Response, error) {
	rq := make(chan []core.Node, 3)
	defer close(rq)
	var rt uint32
	if set.Prefix > 0 {
		rt = set.Prefix
	} else {
		rt = c.Mll.RingToken(set.Data.Key)
		core.AppLog.Debug().Msgf("using key hash %d", rt)
	}
	c.Mll.MRequest <- core.RingRequest{Opt: REPLICA_RING_OPT, Token: rt, Replicas: REPLICA_MAX, Async: rq}
	nodes := <-rq
	ringNode := nodes[0]
	conn, err := ringNode.CPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	}
	dsp := protocol.NewDataServiceClient(conn.Conn)
	return dsp.Get(context.Background(), set)
}
