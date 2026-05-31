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
	if plan.AppRepo == nil || plan.AppRepo.Name == "" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "appRepo name is required"}))
		return
	}
	if plan.Vendor == "" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "vendor is required"}))
		return
	}

	msg, err := anypb.New(&plan)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}

	tb := persistence.NewTaskBuilder(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "cluster-create"})

	// Validator: check repo and cloud resources are ready
	vb := tb.Validator(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "validator"})
	vb.Transaction().Meta(&protocol.Meta{Name: "check"}).Message(msg).Build()
	vb.Build()

	// Job 1: create cloud VM instances
	jb1 := tb.Job(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "create"})
	jb1.Transaction().Meta(&protocol.Meta{Name: "create"}).Message(msg).Build()
	jb1.Build()

	// Job 2: install Docker and upload git SSH key on new VMs
	jb2 := tb.Job(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "update"})
	jb2.Transaction().Meta(&protocol.Meta{Name: "update"}).Message(msg).Build()
	jb2.Build()

	// Job 3: clone repo and build Docker image on VMs
	jb3 := tb.Job(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "build"})
	jb3.Transaction().Meta(&protocol.Meta{Name: "build"}).Message(msg).Build()
	jb3.Build()

	// Job 4: deploy Docker image from source tree config on VMs
	jb4 := tb.Job(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "deploy"})
	jb4.Transaction().Meta(&protocol.Meta{Name: "deploy"}).Message(msg).Build()
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
