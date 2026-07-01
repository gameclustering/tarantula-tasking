package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
)

type AdminWorkspaceServices struct{ *AdminService }

func (s *AdminWorkspaceServices) AccessControl() int32 { return core.ADMIN_ACCESS_CONTROL }

func (s *AdminWorkspaceServices) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid id"}))
		return
	}
	list, err := s.ListServiceAccesses(int32(id))
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	if list == nil {
		list = []ServiceAccessRow{}
	}
	w.Write(util.ToJson(list))
}

type AdminWorkspaceServiceDelete struct{ *AdminService }

func (s *AdminWorkspaceServiceDelete) AccessControl() int32 { return core.ADMIN_ACCESS_CONTROL }

func (s *AdminWorkspaceServiceDelete) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid id"}))
		return
	}
	if err := s.DeleteServiceAccess(int32(id)); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	w.Write(util.ToJson(core.OnSession{Successful: true}))
}

type AdminWorkspaceList struct{ *AdminService }

func (s *AdminWorkspaceList) AccessControl() int32 { return core.ADMIN_ACCESS_CONTROL }

func (s *AdminWorkspaceList) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	list, err := s.ListWorkspaces()
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	if list == nil {
		list = []WorkspaceRow{}
	}
	w.Write(util.ToJson(list))
}

type AdminWorkspaceCreate struct{ *AdminService }

func (s *AdminWorkspaceCreate) AccessControl() int32 { return core.ADMIN_ACCESS_CONTROL }

func (s *AdminWorkspaceCreate) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var ws WorkspaceRow
	if err := json.NewDecoder(r.Body).Decode(&ws); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid request body"}))
		return
	}
	if ws.Name == "" || ws.Platform == "" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "name and platform are required"}))
		return
	}
	if ws.SshUser == "" {
		ws.SshUser = "tarantula"
	}
	if ws.InstanceCount < 1 {
		ws.InstanceCount = 1
	}
	if ws.Settings == nil {
		ws.Settings = map[string]string{}
	}
	id, err := s.SaveWorkspace(&ws)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	w.Write(util.ToJson(map[string]any{"successful": true, "id": id}))
}

type AdminWorkspaceUpdate struct{ *AdminService }

func (s *AdminWorkspaceUpdate) AccessControl() int32 { return core.ADMIN_ACCESS_CONTROL }

func (s *AdminWorkspaceUpdate) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid id"}))
		return
	}
	var ws WorkspaceRow
	if err := json.NewDecoder(r.Body).Decode(&ws); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid request body"}))
		return
	}
	ws.Id = int32(id)
	if ws.InstanceCount < 1 {
		ws.InstanceCount = 1
	}
	if ws.Settings == nil {
		ws.Settings = map[string]string{}
	}
	if err := s.UpdateWorkspace(&ws); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	w.Write(util.ToJson(core.OnSession{Successful: true}))
}

type AdminWorkspaceDelete struct{ *AdminService }

func (s *AdminWorkspaceDelete) AccessControl() int32 { return core.ADMIN_ACCESS_CONTROL }

func (s *AdminWorkspaceDelete) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid id"}))
		return
	}
	if err := s.DeleteWorkspace(int32(id)); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	w.Write(util.ToJson(core.OnSession{Successful: true}))
}

// UpsertWorkspaceSection writes the workspace's settings as a named env section
// in the app's deploy.json, then commits and pushes. This is called at task-issue
// time so cloud step workers find the workspace settings via cfg.Resolve(ws.Name, phase).
// Retries once on non-fast-forward by re-cloning the repo.
func (s *AdminService) UpsertWorkspaceSection(ws *WorkspaceRow, services []ServiceAccessRow, deployRepo *protocol.RepoObject, appName string, gitKey *protocol.AuthKey) error {
	repoPath := filepath.Join("work", deployRepo.Name)
	url := fmt.Sprintf("git@github.com:%s/%s.git", gitKey.Git.Org, deployRepo.Name)
	for attempt := 0; attempt < 2; attempt++ {
		err := s.tryUpsertWorkspaceSection(ws, services, appName, url, repoPath, gitKey)
		if err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "non-fast-forward") {
			return err
		}
		os.RemoveAll(repoPath)
	}
	return fmt.Errorf("git push: non-fast-forward after retry")
}

func (s *AdminService) tryUpsertWorkspaceSection(ws *WorkspaceRow, services []ServiceAccessRow, appName, url, repoPath string, gitKey *protocol.AuthKey) error {
	gc := util.GitClient{
		PrivateKey:  gitKey.Git.Key,
		AuthorName:  "tarantula-admin",
		AuthorEmail: "admin@gameclustering.com",
	}
	if err := gc.CloneOrUpdate(url, repoPath); err != nil {
		return fmt.Errorf("clone: %w", err)
	}

	configPath := filepath.Join(repoPath, ws.Platform, appName, "deploy.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read deploy.json: %w", err)
	}

	var config map[string]core.DeployEnvConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse deploy.json: %w", err)
	}

	settings := make(map[string]string, len(ws.Settings))
	for k, v := range ws.Settings {
		settings[k] = v
	}

	serviceAccesses := make([]core.ServiceAccess, 0, len(services))
	for _, sa := range services {
		serviceAccesses = append(serviceAccesses, core.ServiceAccess{
			Name:            sa.Name,
			VaultAccessName: sa.VaultAccessName,
		})
	}

	deployPhase := core.PhaseConfig{
		SshUser:         ws.SshUser,
		VaultHost:       ws.VaultHost,
		Settings:        settings,
		ServiceAccesses: serviceAccesses,
	}

	config[ws.Name] = core.DeployEnvConfig{
		Deploy: deployPhase,
		Build: core.PhaseConfig{
			BuildHost: ws.BuildHost,
		},
	}

	newData, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(configPath, newData, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	relPath := filepath.Join(ws.Platform, appName, "deploy.json")
	if err := gc.Add(relPath); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	st, err := gc.Status()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(st) == "" {
		return nil
	}
	if err := gc.Commit(fmt.Sprintf("feat: sync workspace %s to %s/%s", ws.Name, ws.Platform, appName)); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	if err := gc.Push(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}
