package main

import (
	"io"
	"net/http"
	"strconv"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/encoding/protojson"
)

type AdminClusterRepoList struct {
	*AdminService
}

func (s *AdminClusterRepoList) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}

func (s *AdminClusterRepoList) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	list, err := s.ListRepos()
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	if list == nil {
		list = []RepoRow{}
	}
	w.Write(util.ToJson(list))
}

type AdminClusterRepoCreate struct {
	*AdminService
}

func (s *AdminClusterRepoCreate) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}

func (s *AdminClusterRepoCreate) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var repo protocol.RepoObject
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	if err := protojson.Unmarshal(body, &repo); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	if repo.Name == "" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "name is required"}))
		return
	}
	if _, err := s.SaveRepo(&repo); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	w.Write(util.ToJson(core.OnSession{Successful: true}))
}

type AdminClusterRepoDelete struct {
	*AdminService
}

func (s *AdminClusterRepoDelete) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}

func (s *AdminClusterRepoDelete) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid id"}))
		return
	}
	if err := s.DeleteRepo(int32(id)); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	w.Write(util.ToJson(core.OnSession{Successful: true}))
}
