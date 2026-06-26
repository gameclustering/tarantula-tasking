package core

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sync"
	"time"

	"gameclustering.com/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

const (
	MIN_POOL_SIZE       int = 1
	MAX_POOL_SIZE       int = 1
	RPC_CONNECT_RETRIES int = 3

	RPC_TICKET_TIMEOUT_SECONDS int = 10
)

type RpcConn struct {
	Conn *grpc.ClientConn
	Seq  int
}

type StreamProxy struct {
	grpc.ClientStream
}

func (r *StreamProxy) SendMsg(m any) error {
	return r.ClientStream.SendMsg(m)
}
func (r *StreamProxy) RecvMsg(m any) error {
	return r.ClientStream.RecvMsg(m)
}

type RpcConnPool struct {
	Target  string
	Tag     string
	NodeId  string
	CACert  []byte // PEM-encoded CA certificate used to verify the server
	MinSize int
	MaxSize int
	index   int
	pool    map[string]*RpcConn
	sync.RWMutex
	Auth Authenticator
}

func (p *RpcConnPool) connect(target string) (*grpc.ClientConn, error) {
	retries := RPC_CONNECT_RETRIES
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(p.CACert)
	creds := credentials.NewTLS(&tls.Config{RootCAs: pool})
	cp := grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  100 * time.Millisecond,
			Multiplier: 1.6,
			Jitter:     0.2,
			MaxDelay:   5 * time.Second,
		},
		MinConnectTimeout: 5 * time.Second,
	}
	kp := keepalive.ClientParameters{
		Time:                30 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: true,
	}
	for {
		tcp, err := grpc.NewClient(target,
			grpc.WithTransportCredentials(creds),
			grpc.WithUnaryInterceptor(p.OnCall),
			grpc.WithStreamInterceptor(p.OnStreaming),
			grpc.WithConnectParams(cp),
			grpc.WithKeepaliveParams(kp),
		)
		if err != nil {
			retries--
			if retries > 0 {
				AppLog.Warn().Msgf("retrying to connect gprc %s with retried times %d", err.Error(), retries)
				time.Sleep(3 * time.Second)
				continue
			}
			return tcp, err
		}
		return tcp, nil
	}
}

func (p *RpcConnPool) Start() {
	p.Lock()
	defer p.Unlock()
	if p.MinSize <= 0 {
		p.MinSize = MIN_POOL_SIZE
	}
	if p.MaxSize <= 0 {
		p.MaxSize = MAX_POOL_SIZE
	}
	p.pool = make(map[string]*RpcConn)
	p.index = 0
}

func (p *RpcConnPool) Conn() (*RpcConn, error) {
	p.Lock()
	defer p.Unlock()
	ckey := fmt.Sprintf("%s_%d", p.Target, p.index)
	c, exists := p.pool[ckey]
	if exists {
		state := c.Conn.GetState()
		if state == connectivity.TransientFailure || state == connectivity.Shutdown {
			c.Conn.Close()
			delete(p.pool, ckey)
			exists = false
		}
	}
	if !exists {
		cx, err := p.connect(p.Target)
		if err != nil {
			return nil, err
		}
		c = &RpcConn{Conn: cx, Seq: p.index}
		p.pool[ckey] = c
	}
	return c, nil
}

func (p *RpcConnPool) Shutdown() {
	p.Lock()
	defer p.Unlock()
	for _, c := range p.pool {
		c.Conn.Close()
	}
	clear(p.pool)
}

func (p *RpcConnPool) Release() {
	p.Lock()
	defer p.Unlock()
	for _, c := range p.pool {
		dsp := protocol.NewPostofficeServiceClient(c.Conn)
		resp, err := dsp.Disconnect(p.setup(context.Background()), &protocol.Topic{Tag: p.Tag, NodeId: p.NodeId})
		if err != nil {
			continue
		}
		AppLog.Debug().Msgf("disconnecting topic %v", resp)
		c.Conn.Close()
	}
	clear(p.pool)
}

func (p *RpcConnPool) OnCall(ctx context.Context, method string, req, replay any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if p.Auth == nil {
		AppLog.Warn().Msgf("no auth setup streaming before :%s", method)
		return invoker(ctx, method, req, replay, cc, opts...)
	}
	err := invoker(p.setup(ctx), method, req, replay, cc, opts...)
	return err
}

func (p *RpcConnPool) OnStreaming(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	if p.Auth == nil {
		AppLog.Warn().Msgf("no auth setup streaming before :%s", method)
		s, err := streamer(ctx, desc, cc, method, opts...)
		return &StreamProxy{s}, err
	}
	s, err := streamer(p.setup(ctx), desc, cc, method, opts...)
	return &StreamProxy{s}, err
}

func (p *RpcConnPool) setup(ctx context.Context) context.Context {
	ticket, _ := p.Auth.CreateTicket(100, 100, ADMIN_ACCESS_CONTROL, RPC_TICKET_TIMEOUT_SECONDS)
	return metadata.NewOutgoingContext(ctx, metadata.Pairs(RPC_TICKET_HEADER, ticket))
}
