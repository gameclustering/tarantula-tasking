package clustering

import (
	context "context"
	"fmt"
	"strings"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runUnregister(sub *protocol.Subscription) (*protocol.Response, error) {
	for _, m := range c.Members() {
		if m.Address() == c.LocalNode().Address(){
			c.Unregister(context.Background(),sub)
			continue
		}
		parts := strings.Split(m.Address(), ":")
		rpcEndpoint := fmt.Sprintf("%s:%d", parts[0], core.RPC_PORT)
		if err := c._runUnregister(rpcEndpoint, sub); err != nil {
			core.AppLog.Debug().Msgf("unregister error subscriotion %s from %s %s", sub.Name, rpcEndpoint, err.Error())
		}
	}
	return &protocol.Response{Successful: true}, nil
}
func (c *DataServiceProvider) _runUnregister(target string, sub *protocol.Subscription) error {
	core.AppLog.Debug().Msgf("unregister subscription %s from %s", sub.Name, target)
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
	_, err = dsp.Unregister(ctx, sub)
	if err != nil {
		return err
	}
	return nil
}
