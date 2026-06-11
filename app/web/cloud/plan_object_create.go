package main

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
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
	cfg, err := loadDeployConfig(plan.DeployRepo, plan.Vendor, gitKey)
	if err != nil {
		return fmt.Errorf("deploy config: %w", err)
	}
	deployPhase := cfg.Resolve(plan.Env, "deploy")

	platformKey, err := v.Cluster().AuthKey(platformVaultKey(plan.Vendor))
	if err != nil {
		return fmt.Errorf("%s auth key: %w", plan.Vendor, err)
	}
	platform, err := newPlatform(plan.Vendor, deployPhase, platformKey)
	if err != nil {
		return fmt.Errorf("platform init: %w", err)
	}
	defer platform.Close()

	for i := 1; i <= deployPhase.InstanceNumber; i++ {
		name := fmt.Sprintf("%s-%02d", deployPhase.Prefix, i)
		if err := platform.Provision(name); err != nil {
			return fmt.Errorf("provision %s: %w", name, err)
		}
		core.AppLog.Info().Msgf("create: instance %s provisioned", name)
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
	cfg, err := loadDeployConfig(plan.DeployRepo, plan.Vendor, gitKey)
	if err != nil {
		core.AppLog.Warn().Msgf("cancel deploy config: %s", err)
		return v.insert(t.Meta)
	}
	deployPhase := cfg.Resolve(plan.Env, "deploy")
	platformKey, err := v.Cluster().AuthKey(platformVaultKey(plan.Vendor))
	if err != nil {
		core.AppLog.Warn().Msgf("cancel platform key: %s", err)
		return v.insert(t.Meta)
	}
	platform, err := newPlatform(plan.Vendor, deployPhase, platformKey)
	if err != nil {
		core.AppLog.Warn().Msgf("cancel platform init: %s", err)
		return v.insert(t.Meta)
	}
	defer platform.Close()

	for i := 1; i <= deployPhase.InstanceNumber; i++ {
		name := fmt.Sprintf("%s-%02d", deployPhase.Prefix, i)
		if err := platform.Remove(name); err != nil {
			core.AppLog.Warn().Msgf("cancel: remove %s: %s", name, err)
		}
	}
	return v.insert(t.Meta)
}
