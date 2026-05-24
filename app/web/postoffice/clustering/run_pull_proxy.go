package clustering

import (
	context "context"

	"gameclustering.com/internal/protocol"
	"google.golang.org/grpc"
)

func (c *DataServiceProvider) runPull(tcp *grpc.ClientConn, set *protocol.Request) (grpc.ServerStreamingClient[protocol.Response], error) {
	dsp := protocol.NewDataServiceClient(tcp)
	return dsp.Pull(context.Background(), set)
}
