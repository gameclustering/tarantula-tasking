package cloud

import (
	"fmt"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewCheck(mgr *bootstrap.AppManager, store *Store) *protocol.TccTransationListener {
	h := &checkHandler{mgr: mgr, store: store}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = h.reserve
	tcc.Confirm = h.confirm
	tcc.Cancel = h.cancel
	return &tcc
}

type checkHandler struct {
	mgr   *bootstrap.AppManager
	store *Store
}

func (h *checkHandler) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("check reserve %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	// Service tasks (no appRepo) skip the GitHub repo existence check.
	if plan.AppRepo == nil || plan.AppRepo.Name == "" {
		return h.store.Insert(t.Meta)
	}
	gitKey, err := h.mgr.Cluster().AuthKey("git")
	if err != nil {
		return fmt.Errorf("git auth key: %w", err)
	}
	gapi := util.GitHubApi{Token: gitKey.Git.Token, Org: gitKey.Git.Org}
	repos, err := gapi.ListRepos()
	if err != nil {
		return fmt.Errorf("list repos: %w", err)
	}
	for _, r := range repos {
		if r.Name == plan.AppRepo.Name {
			core.AppLog.Debug().Msgf("repository %s found", plan.AppRepo.Name)
			return h.store.Insert(t.Meta)
		}
	}
	return fmt.Errorf("repository %s not found in org", plan.AppRepo.Name)
}

func (h *checkHandler) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("check confirm %v", t.Meta)
	return h.store.Insert(t.Meta)
}

func (h *checkHandler) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("check cancel %v", t.Meta)
	return h.store.Insert(t.Meta)
}
