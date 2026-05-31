package main

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewRepositoryObejctCheck(s *CloudService) *protocol.TccTransationListener {
	c := RepositoryObejctCheck{s}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = c.reserve
	tcc.Confirm = c.confirm
	tcc.Cancel = c.cancel
	return &tcc
}

type RepositoryObejctCheck struct {
	*CloudService
}

func (v *RepositoryObejctCheck) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("check reserve %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	gitKey, err := v.Cluster().AuthKey("git")
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
			return v.insert(t.Meta)
		}
	}
	return fmt.Errorf("repository %s not found in org", plan.AppRepo.Name)
}

func (v *RepositoryObejctCheck) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("check confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *RepositoryObejctCheck) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("check cancel %v", t.Meta)
	return v.insert(t.Meta)
}
