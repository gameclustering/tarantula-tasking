package clustering

import (
	"context"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

const CLIENT_TIMEOUT = 10 * time.Second

func (c *DataServiceProvider) runCreate(set *protocol.Request) (*protocol.Response, error) {
	rq := make(chan []core.Node, 3)
	defer close(rq)
	retry := RetryTrack{Reties: RETRY_MAX}
	var rt uint32
	if set.Prefix > 0 {
		rt = set.Prefix
	} else {
		rt = c.Mll.RingToken(set.Data.Key)
		core.AppLog.Debug().Msgf("using key hash %d", rt)
	}
	for retry.Reties > 0 {
		c.Mll.MRequest <- core.RingRequest{Opt: REPLICA_RING_OPT, Token: rt, Replicas: REPLICA_MAX, Async: rq}
		nodes := <-rq
		ringNode := nodes[0]
		resp, err := c.clientCreate(&ringNode, set)
		if err != nil {
			core.AppLog.Warn().Msgf("primary create failed node=%s err=%s", ringNode.RpcEndpoint, err.Error())
			retry.Reties--
			continue
		}
		if !resp.Successful {
			retry.Err = resp.Message
			retry.Reties--
			continue
		}
		retry.Suc = true
		// replicate to slaves asynchronously — primary success is sufficient for the caller
		slaves := nodes[1:]
		for _, slave := range slaves {
			s := slave
			go func() {
				if _, err := c.clientCreate(&s, set); err != nil {
					core.AppLog.Debug().Msgf("slave replication error %s", err.Error())
				}
			}()
		}
		break
	}
	return &protocol.Response{Successful: retry.Suc, Message: retry.Err}, nil
}

func (m *DataServiceProvider) clientCreate(target *core.Node, request *protocol.Request) (*protocol.Response, error) {
	if target.RpcEndpoint == m.rpcEndpoint {
		return m.Create(context.Background(), request)
	}
	conn, err := target.CPool.Conn()
	if err != nil {
		return &protocol.Response{Successful: false, Message: "no tcp connect"}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), CLIENT_TIMEOUT)
	defer cancel()
	dsp := protocol.NewDataServiceClient(conn.Conn)
	return dsp.Create(ctx, request)
}
