package clustering

import (
	context "context"
	"io"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/grpc"
)

func (c *DataServiceProvider) runQuery(set *protocol.Request, responser grpc.ServerStreamingServer[protocol.Response]) {
	rq := make(chan []core.Node, 3)
	defer close(rq)
	c.MRequest <- core.RingRequest{Opt: REPLICA_RING_OPT, Token: set.Prefix, Replicas: REPLICA_MAX, Async: rq}
	nodes := <-rq
	ringNode := nodes[0]
	conn, err := ringNode.CPool.Conn()
	if err != nil {
		responser.Send(&protocol.Response{Successful: false, Message: err.Error()})
		return
	}
	dsp := protocol.NewDataServiceClient(conn.Conn)
	stream, err := dsp.Query(context.Background(), set)
	if err != nil {
		responser.Send(&protocol.Response{Successful: false, Message: err.Error(), Code: 500000})
		return
	}
	for {
		data, err := stream.Recv()
		if err == io.EOF {
			responser.Send(&protocol.Response{Successful: false, Message: err.Error(), Code: 500000})
			break
		}
		if err != nil {
			core.AppLog.Warn().Msgf("run get streaming error %s", err.Error())
			responser.Send(&protocol.Response{Successful: false, Message: err.Error(), Code: 500000})
			break
		}
		responser.Send(data)
		if !data.Successful {
			break
		}
	}

}
