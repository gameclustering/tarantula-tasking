package clustering

import (
	"fmt"
	"strings"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func (c *DataServiceProvider) runRegister(sub *protocol.Subscription) (*protocol.Response, error) {
	for _, m := range c.Members() {
		parts := strings.Split(m.Address(), ":")
		rpcEndpint := fmt.Sprintf("%s:%d", parts[0], core.RPC_PORT)
		core.AppLog.Debug().Msgf("register on to %s", rpcEndpint)
		
	}
	return &protocol.Response{Successful: true}, nil
}
