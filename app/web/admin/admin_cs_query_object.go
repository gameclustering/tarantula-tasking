package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
)

type CSQueryObject struct {
	*AdminService
}

func (s *CSQueryObject) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}

func (s *CSQueryObject) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	topic := r.PathValue("topic")
	mc, existed := core.QueryFactoryRegistry[topic]
	if !existed {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: fmt.Sprintf("topic %s not existed", topic)}))
		return
	}
	mf, ok := mc().(core.ProtoObjectFactory)
	if !ok {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: fmt.Sprintf("proto factory %s not existed", topic)}))
		return
	}
	me := mf.Query()
	err := json.NewDecoder(r.Body).Decode(&me)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	//req := core.DataRequest{Opt: core.QUERY_DATA_REQUEST, Criteria: me}
	stream, err := s.Cluster().List(me)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	ms := make([]proto.Message, 0)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			core.AppLog.Warn().Msgf("streaming error %s", err.Error())
			break
		}
		mf.Set(resp).QList(func(h *protocol.Header, m proto.Message) bool {
			ms = append(ms, m)
			return true
		})
	}
	w.Write(util.ToJson(ms))
}
