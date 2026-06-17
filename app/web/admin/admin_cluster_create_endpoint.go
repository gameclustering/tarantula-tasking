package main

import (
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
	return core.ADMIN_ACCESS_CONTROL
}

func (s *AdminClusterCreate) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var plan protocol.PlanObject
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	if err := protojson.Unmarshal(body, &plan); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	if plan.Platform == "" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "platform is required"}))
		return
	}
	// App tasks require an appRepo; service tasks require a name instead.
	if (plan.AppRepo == nil || plan.AppRepo.Name == "") && plan.Name == "" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "appRepo or service name is required"}))
		return
	}

	msg, err := anypb.New(&plan)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}

	p := plan.Platform
	tb := persistence.NewTaskBuilder(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "cluster-create"})

	vb := tb.Validator(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "validator"})
	vb.Transaction().Meta(&protocol.Meta{Name: "check_" + p}).Message(msg).Build()
	vb.Build()

	jb1 := tb.Job(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "create"})
	jb1.Transaction().Meta(&protocol.Meta{Name: "create_" + p}).Message(msg).Build()
	jb1.Build()

	jb2 := tb.Job(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "update"})
	jb2.Transaction().Meta(&protocol.Meta{Name: "update_" + p}).Message(msg).Build()
	jb2.Build()

	jb3 := tb.Job(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "build"})
	jb3.Transaction().Meta(&protocol.Meta{Name: "build_" + p}).Message(msg).Build()
	jb3.Build()

	jb4 := tb.Job(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "deploy"})
	jb4.Transaction().Meta(&protocol.Meta{Name: "deploy_" + p}).Message(msg).Build()
	jb4.Build()

	rp, err := s.Cluster().Issue(tb.Build())
	if err != nil {
		core.AppLog.Debug().Msgf("cluster create task error: %s", err.Error())
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	core.AppLog.Debug().Msgf("cluster create task issued: %v", rp)
	w.Write(util.ToJson(core.OnSession{Successful: true}))
}
