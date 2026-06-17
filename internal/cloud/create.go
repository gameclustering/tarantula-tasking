package cloud

import (
	"fmt"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewCreate(mgr *bootstrap.AppManager, store *Store, vaultKey string, factory PlatformFactory) *protocol.TccTransationListener {
	h := &createHandler{mgr: mgr, store: store, vaultKey: vaultKey, factory: factory}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = h.reserve
	tcc.Confirm = h.confirm
	tcc.Cancel = h.cancel
	return &tcc
}

type createHandler struct {
	mgr      *bootstrap.AppManager
	store    *Store
	vaultKey string
	factory  PlatformFactory
}

func (h *createHandler) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("create reserve %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	gitKey, err := h.mgr.Cluster().AuthKey("git")
	if err != nil {
		return fmt.Errorf("git auth key: %w", err)
	}
	planName := plan.Name
	if planName == "" && plan.AppRepo != nil {
		planName = plan.AppRepo.Name
	}
	cfg, err := LoadDeployConfig(plan.DeployRepo, plan.Platform, planName, gitKey)
	if err != nil {
		return fmt.Errorf("deploy config: %w", err)
	}
	deployPhase := cfg.Resolve(plan.Env, "deploy")

	platformKey, err := h.mgr.Cluster().AuthKey(h.vaultKey)
	if err != nil {
		return fmt.Errorf("%s auth key: %w", h.vaultKey, err)
	}
	platform, err := h.factory(deployPhase, platformKey)
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
	return h.store.Insert(t.Meta)
}

func (h *createHandler) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("create confirm %v", t.Meta)
	return h.store.Insert(t.Meta)
}

func (h *createHandler) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("create cancel %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
		core.AppLog.Warn().Msgf("cancel unmarshal: %s", err)
		return h.store.Insert(t.Meta)
	}
	gitKey, err := h.mgr.Cluster().AuthKey("git")
	if err != nil {
		core.AppLog.Warn().Msgf("cancel git auth key: %s", err)
		return h.store.Insert(t.Meta)
	}
	cancelName := plan.Name
	if cancelName == "" && plan.AppRepo != nil {
		cancelName = plan.AppRepo.Name
	}
	cfg, err := LoadDeployConfig(plan.DeployRepo, plan.Platform, cancelName, gitKey)
	if err != nil {
		core.AppLog.Warn().Msgf("cancel deploy config: %s", err)
		return h.store.Insert(t.Meta)
	}
	deployPhase := cfg.Resolve(plan.Env, "deploy")
	platformKey, err := h.mgr.Cluster().AuthKey(h.vaultKey)
	if err != nil {
		core.AppLog.Warn().Msgf("cancel platform key: %s", err)
		return h.store.Insert(t.Meta)
	}
	platform, err := h.factory(deployPhase, platformKey)
	if err != nil {
		core.AppLog.Warn().Msgf("cancel platform init: %s", err)
		return h.store.Insert(t.Meta)
	}
	defer platform.Close()

	for i := 1; i <= deployPhase.InstanceNumber; i++ {
		name := fmt.Sprintf("%s-%02d", deployPhase.Prefix, i)
		if err := platform.Remove(name); err != nil {
			core.AppLog.Warn().Msgf("cancel: remove %s: %s", name, err)
		}
	}
	return h.store.Insert(t.Meta)
}
