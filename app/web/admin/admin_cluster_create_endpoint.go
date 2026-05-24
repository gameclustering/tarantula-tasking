package main

import (
	"bytes"
	"io"
	"net/http"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
)

type AdminClusterCreate struct {
	*AdminService
}

func (s *AdminClusterCreate) AccessControl() int32 {
	return core.PROTECTED_ACCESS_CONTROL
}

func (s *AdminClusterCreate) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var me protocol.RepositoryObject
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
	repo, err := anypb.New(&me)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	vm, err := anypb.New(&protocol.VMObject{ProjectId: "test", Zone: "zone1"})
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	tb := persistence.NewTaskBuilder(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "register"})
	vb := tb.Validator(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "validator"})
	vb.Transaction().Meta(&protocol.Meta{Name: "check"}).Message(repo).Build()
	vb.Transaction().Meta(&protocol.Meta{Name: "update"}).Message(vm).Build()
	vb.Transaction().Meta(&protocol.Meta{Name: "create"}).Message(vm).Build()
	jb := tb.Job(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "job"})
	jb.Transaction().Meta(&protocol.Meta{Name: "update"}).Message(vm).Build()
	jb.Transaction().Meta(&protocol.Meta{Name: "create"}).Message(vm).Build()
	jb.Transaction().Meta(&protocol.Meta{Name: "create"}).Message(vm).Build()
	jb.Build()
	rp, err := s.Cluster().Issue(tb.Build())
	if err != nil {
		core.AppLog.Debug().Msgf("TASK ERR %s", err.Error())
		return
	}
	core.AppLog.Debug().Msgf("TASK %v", rp)
	w.Write(util.ToJson(core.OnSession{Successful: true, Message: ""}))
}
