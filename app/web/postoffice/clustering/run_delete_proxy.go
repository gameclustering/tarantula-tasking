package clustering

import (
	context "context"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runDelete(set *protocol.Request) (*protocol.Response, error) {
	rq := make(chan []core.Node, 3)
	defer close(rq)
	var rt uint32
	if set.Prefix > 0 {
		rt = set.Prefix
	} else {
		rt = c.Mll.RingToken(set.Data.Key)
		core.AppLog.Debug().Msgf("using key hash %d", rt)
	}
	retry := RetryTrack{Reties: RETRY_MAX}
	for retry.Reties > 0 {
		c.MRequest <- core.RingRequest{Opt: REPLICA_RING_OPT, Token: rt, Replicas: REPLICA_MAX, Async: rq}
		nodes := <-rq
		ringNode := nodes[0]
		resp, _ := c.clientDelete(&ringNode, set)
		if !resp.Successful {
			retry.Err = resp.Message
			retry.Reties--
			continue
		}
		retry.Suc = true
		slaves := nodes[1:]
		for _, slave := range slaves {
			c.clientDelete(&slave, set)
		}
		break
	}
	return &protocol.Response{Successful: retry.Suc, Message: retry.Err}, nil
}

func (m *DataServiceProvider) clientDelete(target *core.Node, request *protocol.Request) (*protocol.Response, error) {
	if target.RpcEndpoint == m.rpcEndpoint {
		return m.Delete(context.Background(), request)
	}
	conn, err := target.CPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	}
	dsp := protocol.NewDataServiceClient(conn.Conn)
	return dsp.Delete(context.Background(), request)
}
