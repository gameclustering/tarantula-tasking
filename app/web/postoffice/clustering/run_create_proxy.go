package clustering

import (
	context "context"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runCreate(set *protocol.Request) (*protocol.Response, error) {
	rq := make(chan []core.Node, 3)
	defer close(rq)
	retry := RetryTrack{Reties: RETRY_MAX}
	var rt uint32
	if set.Prefix > 0 {
		rt = set.Prefix
	} else {	
		rt = c.Mll.RingToken(set.Data.Key)
		core.AppLog.Debug().Msgf("using key hash %d",rt)
	}
	for retry.Reties > 0 {
		c.Mll.MRequest <- core.RingRequest{Opt: REPLICA_RING_OPT, Token: rt, Replicas: REPLICA_MAX, Async: rq}
		nodes := <-rq
		ringNode := nodes[0]
		resp, err := c.clientCreate(&ringNode, set)
		if err != nil {
			retry.Reties--
			continue
		}
		if !resp.Successful {
			retry.Err = resp.Message
			retry.Reties--
			continue
		}
		retry.Suc = true
		slaves := nodes[1:]
		for _, slave := range slaves {
			_, err := c.clientCreate(&slave, set)
			if err != nil {
				core.AppLog.Debug().Msgf("error on slave %s", err.Error())
			}
		}
		break
	}
	return &protocol.Response{Successful: retry.Suc, Message: retry.Err}, nil
}

func (m *DataServiceProvider) clientCreate(target *core.Node, request *protocol.Request) (*protocol.Response, error) {
	conn, err := target.CPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false, Message: "no tcp connect"}, err
	}
	dsp := protocol.NewDataServiceClient(conn.Conn)
	return dsp.Create(context.Background(), request)
}
