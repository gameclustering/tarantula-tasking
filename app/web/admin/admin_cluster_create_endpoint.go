package main

import (
	"io"
	"net/http"

	"gameclustering.com/internal/cloud"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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
	if (plan.AppRepo == nil || plan.AppRepo.Name == "") && plan.Name == "" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "appRepo or service name is required"}))
		return
	}

	gitKey, err := s.Cluster().AuthKey("git")
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "git auth key: " + err.Error()}))
		return
	}
	planName := plan.Name
	if planName == "" && plan.AppRepo != nil {
		planName = plan.AppRepo.Name
	}
	cfg, err := cloud.LoadDeployConfig(plan.DeployRepo, plan.Platform, planName, gitKey)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "deploy config: " + err.Error()}))
		return
	}

	steps := cfg.ResolveSteps(plan.Env)
	deployPhase := cfg.Resolve(plan.Env, "deploy")
	instanceCount := deployPhase.InstanceNumber
	if instanceCount < 1 {
		instanceCount = 1
	}

	p := plan.Platform
	tb := persistence.NewTaskBuilder(&protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: "cluster-create"})

	for i, step := range steps {
		meta := &protocol.Meta{NodeId: s.NodeId(), Tag: s.Context(), Name: step}
		if i == 0 {
			// First step is always the validator.
			msg, err := anypb.New(&plan)
			if err != nil {
				w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
				return
			}
			vb := tb.Validator(meta)
			vb.Transaction().Meta(&protocol.Meta{Name: step + "_" + p}).Message(msg).Build()
			vb.Build()
			continue
		}
		jb := tb.Job(meta)
		if core.ParallelSteps[step] {
			// Fan-out: one transaction per instance so each subscriber handles exactly one VM.
			for seq := 1; seq <= instanceCount; seq++ {
				seqPlan := proto.Clone(&plan).(*protocol.PlanObject)
				seqPlan.Seq = int32(seq)
				msg, err := anypb.New(seqPlan)
				if err != nil {
					w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
					return
				}
				jb.Transaction().Meta(&protocol.Meta{Name: step + "_" + p}).Message(msg).Build()
			}
		} else {
			msg, err := anypb.New(&plan)
			if err != nil {
				w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
				return
			}
			jb.Transaction().Meta(&protocol.Meta{Name: step + "_" + p}).Message(msg).Build()
		}
		jb.Build()
	}

	rp, err := s.Cluster().Issue(tb.Build())
	if err != nil {
		core.AppLog.Debug().Msgf("cluster create task error: %s", err.Error())
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	core.AppLog.Debug().Msgf("cluster create task issued: %v", rp)
	w.Write(util.ToJson(core.OnSession{Successful: true}))
}
