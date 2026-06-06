package main

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewPlanObjectCreate(s *CloudService) *protocol.TccTransationListener {
	p := PlanObjectCreate{s}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = p.reserve
	tcc.Confirm = p.confirm
	tcc.Cancel = p.cancel
	return &tcc
}

type PlanObjectCreate struct {
	*CloudService
}

func (v *PlanObjectCreate) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("create reserve %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	gitKey, err := v.Cluster().AuthKey("git")
	if err != nil {
		return fmt.Errorf("git auth key: %w", err)
	}
	cfg, err := loadGcpDeployConfig(plan.DeployRepo, gitKey)
	if err != nil {
		return fmt.Errorf("deploy config: %w", err)
	}
	phase := cfg.Resolve(plan.Env, "deploy")

	gcpKey, err := v.Cluster().AuthKey("gcp")
	if err != nil {
		return fmt.Errorf("gcp auth key: %w", err)
	}
	gcp := util.GcpApi{ServiceAccount: gcpKey.Gcp.Iam, ProjectId: gcpKey.Gcp.ProjectId, Zone: phase.Zone}
	if err := gcp.Auth(); err != nil {
		return fmt.Errorf("gcp auth: %w", err)
	}
	defer gcp.Close()

	for i := 1; i <= phase.InstanceNumber; i++ {
		name := fmt.Sprintf("%s-%02d", phase.Prefix, i)
		if _, err := gcp.Get(name); err == nil {
			core.AppLog.Info().Msgf("instance %s already exists, skipping", name)
			continue
		}
		core.AppLog.Info().Msgf("creating instance %s", name)
		if err := gcp.Insert(name, phase.MachineType, phase.ImageType); err != nil {
			return fmt.Errorf("create instance %s: %w", name, err)
		}
		core.AppLog.Info().Msgf("instance %s created", name)
	}
	return v.insert(t.Meta)
}

func (v *PlanObjectCreate) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("create confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *PlanObjectCreate) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("create cancel %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
		core.AppLog.Warn().Msgf("cancel unmarshal: %s", err)
		return v.insert(t.Meta)
	}
	gitKey, err := v.Cluster().AuthKey("git")
	if err != nil {
		core.AppLog.Warn().Msgf("cancel git auth key: %s", err)
		return v.insert(t.Meta)
	}
	cfg, err := loadGcpDeployConfig(plan.DeployRepo, gitKey)
	if err != nil {
		core.AppLog.Warn().Msgf("cancel deploy config: %s", err)
		return v.insert(t.Meta)
	}
	phase := cfg.Resolve(plan.Env, "deploy")

	gcpKey, err := v.Cluster().AuthKey("gcp")
	if err != nil {
		core.AppLog.Warn().Msgf("cancel gcp auth key: %s", err)
		return v.insert(t.Meta)
	}
	gcp := util.GcpApi{ServiceAccount: gcpKey.Gcp.Iam, ProjectId: gcpKey.Gcp.ProjectId, Zone: phase.Zone}
	if err := gcp.Auth(); err != nil {
		core.AppLog.Warn().Msgf("cancel gcp auth: %s", err)
		return v.insert(t.Meta)
	}
	defer gcp.Close()

	for i := 1; i <= phase.InstanceNumber; i++ {
		name := fmt.Sprintf("%s-%02d", phase.Prefix, i)
		core.AppLog.Info().Msgf("deleting instance %s (cancel rollback)", name)
		if err := gcp.Delete(name); err != nil {
			core.AppLog.Warn().Msgf("delete instance %s: %s", name, err)
		}
	}
	return v.insert(t.Meta)
}
