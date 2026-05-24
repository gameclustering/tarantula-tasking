package main

import (
	"bytes"
	"io"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/event"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
)

type CSMessager struct {
	*AdminService
}

func (s *CSMessager) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}
func (s *CSMessager) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	mf := event.MessageEventFactory{}
	var me protocol.MessageEvent
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r.Body)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	err = protojson.Unmarshal(buf.Bytes(), &me)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	core.AppLog.Debug().Msgf("event %v", &me)
	tp, err := mf.FromMessage(&me, mf.Header(event.MESSAGE_EVENT_CID))
	tp.Name = event.MESSAGE_TOPIC_NAME
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	tp.Event.Key.Array = core.ToBytes(s.Sequence())
	tp.NodeId = s.NodeId()
	tp.Tag = s.Context()
	resp, err := s.Cluster().Publish(tp)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	w.Write(util.ToJson(core.OnSession{Successful: true, Message: resp.Message}))
}
