package clustering

import (
	"context"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runNotifyRingSync(target string, ring *protocol.RingSync) error {
	core.AppLog.Debug().Msgf("notify ring sync to %s", target)
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
	_, err = dsp.NotifyRingSync(ctx, ring)
	if err != nil {
		return err
	}
	return nil
}
