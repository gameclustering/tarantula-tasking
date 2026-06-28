package clustering

import (
	context "context"
	"fmt"
	"strings"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runRemoveSubByNodeId(sub *protocol.Subscription) (*protocol.Response, error) {
	for _, m := range c.Members() {
		if m.Address() == c.LocalNode().Address() {
			c.RemoveSubsByNodeId(context.Background(), sub)
			continue
		}
		parts := strings.Split(m.Address(), ":")
		rpcEndpoint := fmt.Sprintf("%s:%d", parts[0], core.RPC_PORT)
		if err := c._runRemoveSubByNodeId(rpcEndpoint, sub); err != nil {
			core.AppLog.Debug().Msgf("remove subscriotions error %s from %s %s", sub.NodeId, rpcEndpoint, err.Error())
		}
	}
	return &protocol.Response{Successful: true}, nil
}
func (c *DataServiceProvider) _runRemoveSubByNodeId(target string, sub *protocol.Subscription) error {
	core.AppLog.Debug().Msgf("remove subscriptions %s from %s", sub.NodeId, target)
	cpool := core.RpcConnPool{Target: target, Auth: c.auth, CACert: c.caCert}
	cpool.Start()
	conn, err := cpool.Conn()
	if err != nil {
		return err
	}
	defer cpool.Shutdown()
	dsp := protocol.NewDataServiceClient(conn.Conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = dsp.RemoveSubsByNodeId(ctx, sub)
	if err != nil {
		return err
	}
	return nil
}
