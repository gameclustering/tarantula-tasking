package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type CSQueryTopic struct {
	*AdminService
}

func (s *CSQueryTopic) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}

func (s *CSQueryTopic) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	topic := r.PathValue("topic")
	mc, existed := core.QueryFactoryRegistry[topic]
	if !existed {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: fmt.Sprintf("topic %s not existed", topic)}))
		return
	}
	mf, ok := mc().(core.ProtoTopicFactory)
	if !ok {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "proto factory not existed"}))
		return
	}
	me := mf.Query()
	err := json.NewDecoder(r.Body).Decode(&me)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	stream, err := s.Cluster().List(me)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	ms := make([]*anypb.Any, 0)
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
			r, err := anypb.New(m)
			if err == nil {
				ms = append(ms, r)
			}
			return true
		})
	}
	data, err := protojson.Marshal(&protocol.Response{Messages: ms})
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	w.Write(data)
}
