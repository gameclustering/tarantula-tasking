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
			continue
		}
		dsp := protocol.NewDataServiceClient(conn.Conn)
		dsp.Regsiter(context.Background(), sub)
		cpool.Shutdown()
	}
	return &protocol.Response{Successful: true}, nil
}
