package clustering

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runSyncSubs(sub *protocol.SubsSync) (*protocol.Response, error) {
	for _, m := range c.Members() {
		if c.LocalNode().Address() == m.Address() {
			continue
		}
		parts := strings.Split(m.Address(), ":")
		rpcEndpoint := fmt.Sprintf("%s:%d", parts[0], core.RPC_PORT)
		if err := c._runSyncSubs(rpcEndpoint, sub); err != nil {
			core.AppLog.Debug().Msgf("sub sync error %s on %s", rpcEndpoint, err.Error())
		}
	}
	return &protocol.Response{Successful: true}, nil
}

func (c *DataServiceProvider) _runSyncSubs(target string, sub *protocol.SubsSync) error {
	core.AppLog.Debug().Msgf("sync subs on to %s", target)
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
	_, err = dsp.SyncSubs(ctx, sub)
	if err != nil {
		return err
	}
	return nil
}
