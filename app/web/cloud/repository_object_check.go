package main

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewRepositoryObejctCheck(s *CloudService) *protocol.TccTransationListener {
	vm := RepositoryObejctCheck{s}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = vm.reserse
	tcc.Confirm = vm.confirm
	tcc.Cancel = vm.cancel
	return &tcc
}

type RepositoryObejctCheck struct {
	*CloudService
}

func (v *RepositoryObejctCheck) reserse(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update reserve %v", t.Meta)
	var repo protocol.RepositoryObject
	err := anypb.UnmarshalTo(t.Message, &repo, proto.UnmarshalOptions{})
	if err != nil {
		return err
	}

	core.AppLog.Debug().Msgf("repository object %v", &repo)
	github, err := v.AppManager.Cluster().AuthKey("git")
	if err != nil {
		core.AppLog.Debug().Msgf("no git key %s", err.Error())
		return v.insert(t.Meta)
	}
	gapi := util.GitHubApi{Token: github.Git.Token, Org: github.Git.Org}
	repos, err := gapi.ListRepos()
	if err != nil {
		core.AppLog.Debug().Msgf("repo error %s", err.Error())
	}
	core.AppLog.Debug().Msgf("repos %v", repos)
	return v.insert(t.Meta)
}

func (v *RepositoryObejctCheck) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("check confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *RepositoryObejctCheck) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("check cancel %v", t.Meta)
	return v.insert(t.Meta)
}
