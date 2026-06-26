package clustering

import (
	"context"
	"fmt"
	"strings"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runRegister(sub *protocol.Subscription) (*protocol.Response, error) {
	for _, m := range c.Members() {
		parts := strings.Split(m.Address(), ":")
		rpcEndpoint := fmt.Sprintf("%s:%d", parts[0], core.RPC_PORT)
		core.AppLog.Debug().Msgf("register on to %s", rpcEndpoint)
		cpool := core.RpcConnPool{Target: rpcEndpoint, Auth: c.auth, CACert: c.caCert}
		cpool.Start()
		conn, err := cpool.Conn()
		if err != nil {
			core.AppLog.Debug().Msgf("connect failed %s", err.Error())
			continue
		}
		defer cpool.Shutdown()
		dsp := protocol.NewDataServiceClient(conn.Conn)
		resp, err := dsp.Register(context.Background(), sub)
		if err != nil {
			core.AppLog.Debug().Msgf("connect failed %s", err.Error())
			continue
		}
		core.AppLog.Debug().Msgf("resgiter status %v", resp.Successful)
	}
	return &protocol.Response{Successful: true}, nil
}
