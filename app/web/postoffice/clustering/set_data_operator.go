package clustering

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	SET_OPERATOR_NUM int = 8
)

func (m *DataServiceProvider) runSetData(num int) {
start:
	m.DWait.Wait()
	core.AppLog.Info().Msgf("starting set operator %d", num)
	for sd := range m.DSet {
		if sd.Opt == core.SET_OPT_RECOVER {
			break
		} else if sd.Opt == core.SET_OPT_CLOSE {
			core.AppLog.Debug().Msgf("closing set data operator %d", num)
			return
		}
		switch sd.Opt {
		case core.CREATE_DATA_REQUEST:
			ki, err := m.create(sd)
			if err != nil {
				sd.Resp <- &protocol.Response{Successful: false, Message: err.Error()}
			} else {
				var data []*protocol.Data
				data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
				sd.Resp <- &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}
			}
		case core.UPDATE_DATA_REQUEST:
			ki, err := m.update(sd)
			if err != nil {
				sd.Resp <- &protocol.Response{Successful: false, Message: err.Error()}
			} else {
				var data []*protocol.Data
				data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
				sd.Resp <- &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}
			}
		case core.DELETE_DATA_REQUEST:
			ki, err := m.delete(sd)
			if err != nil {
				sd.Resp <- &protocol.Response{Successful: false, Message: err.Error()}
			} else {
				var data []*protocol.Data
				data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
				sd.Resp <- &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}
			}
		case core.RESET_DATA_REQUEST:
			ki, err := m.reset(sd)
			if err != nil {
				sd.Resp <- &protocol.Response{Successful: false, Message: err.Error()}
			} else {
				var data []*protocol.Data
				data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
				sd.Resp <- &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}
			}
		default:
			sd.Resp <- &protocol.Response{Successful: false, Message: fmt.Sprintf("set opt not supported %d", sd.Opt)}
		}
	}
	core.AppLog.Info().Msgf("running recovery on operator %d", num)
	sync := <-m.DPull
	total := 0
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(m.CACert)
	creds := credentials.NewTLS(&tls.Config{RootCAs: pool})
	p := core.RpcConnPool{Auth: m.auth}
	tcp, err := grpc.NewClient(sync.Remote, grpc.WithTransportCredentials(creds),grpc.WithUnaryInterceptor(p.OnCall), grpc.WithStreamInterceptor(p.OnStreaming))
	if err != nil {
		core.AppLog.Warn().Msgf("rpc connect error %s from %s", err.Error(), sync.Remote)
		m.DWait.Done()
		goto start
	}
	for _, h := range sync.Ranges {
		req := protocol.Request{Prefix: h.From, Opt: h.To}
		stream, err := m.runPull(tcp, &req)
		if err != nil {
			core.AppLog.Warn().Msgf("remote error %s", sync.Remote)
			continue
		}
		for {
			data, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				core.AppLog.Warn().Msgf("remote streaming error %s %s %d", sync.Remote, err.Error(), num)
				break
			}
			total += len(data.Data.List)
			m.set(data)
		}
	}
	core.AppLog.Info().Msgf("total data rows %d on %d", total, num)
	tcp.Close()
	m.DWait.Done()
	goto start
}
