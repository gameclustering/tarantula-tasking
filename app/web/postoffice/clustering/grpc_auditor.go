package clustering

import (
	context "context"

	"gameclustering.com/internal/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ServerStreamPorxy struct {
	grpc.ServerStream
}

func (r *ServerStreamPorxy) SendMsg(m any) error {
	return r.ServerStream.SendMsg(m)
}

func (r *ServerStreamPorxy) RecvMsg(m any) error {
	return r.ServerStream.RecvMsg(m)
}

func (c *DataServiceProvider) auditCall(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	hd, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no ticket header")
	}
	ticks := hd.Get(core.RPC_TICKET_HEADER)
	if len(ticks) == 0 {
		return nil, status.Error(codes.Unauthenticated, "no ticket header")
	}
	_, err := c.auth.ValidateTicket(ticks[0])
	if err != nil {
		//return nil, status.Error(codes.Unauthenticated, "invalid ticket")
		core.AppLog.Debug().Msgf("invlid ticket %s", err.Error())
	}
	return handler(ctx, req)
}

func (c *DataServiceProvider) auditStreaming(req any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	hd, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "no ticket header")
	}
	ticks := hd.Get(core.RPC_TICKET_HEADER)
	if len(ticks) == 0 {
		return status.Error(codes.Unauthenticated, "no ticket header")
	}
	_, err := c.auth.ValidateTicket(ticks[0])
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "invalid ticket %s", err.Error())
	}
	err = handler(req, &ServerStreamPorxy{ss})
	return err
}
