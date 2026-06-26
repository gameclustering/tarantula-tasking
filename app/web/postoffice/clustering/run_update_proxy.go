package clustering

import (
	context "context"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runUpdate(set *protocol.Request) (*protocol.Response, error) {
	rq := make(chan []core.Node, 3)
	defer close(rq)
	var rt uint32
	if set.Prefix > 0 {
		rt = set.Prefix
	} else {
		rt = c.RingToken(set.Data.Key)
		core.AppLog.Debug().Msgf("using key hash %d", rt)
	}
	retry := RetryTrack{Reties: RETRY_MAX}
	var mresp *protocol.Response
	for retry.Reties > 0 {
		c.MRequest <- core.RingRequest{Opt: REPLICA_RING_OPT, Token: rt, Replicas: REPLICA_MAX, Async: rq}
		nodes := <-rq
		ringNode := nodes[0]
		resp, _ := c.clientUpdate(&ringNode, set)
		if !resp.Successful {
			retry.Err = resp.Message
			retry.Reties--
			continue
		}
		mresp = resp
		retry.Suc = true
		slaves := nodes[1:]
		for _, slave := range slaves {
			c.clientUpdate(&slave, set)
		}
		break
	}
	if retry.Suc {
		return mresp, nil
	}
	return &protocol.Response{Successful: false, Message: retry.Err}, nil
}

func (m *DataServiceProvider) clientUpdate(target *core.Node, request *protocol.Request) (*protocol.Response, error) {
	if target.RpcEndpoint == m.rpcEndpoint {
		return m.Update(context.Background(), request)
	}
	conn, err := target.CPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false, Message: err.Error()}, err
	}
	dsp := protocol.NewDataServiceClient(conn.Conn)
	return dsp.Update(context.Background(), request)
}
