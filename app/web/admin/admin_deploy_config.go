package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

type deployConfigService struct {
	Name        string `json:"name"`
	Network     string `json:"network"`
	HttpBinding string `json:"httpBinding"`
}

type deployConfigRequest struct {
	AppName    string                `json:"appName"`
	Platform   string                `json:"platform"`
	DeployRepo string                `json:"deployRepo"`
	Prefix     string                `json:"prefix"`
	Services   []deployConfigService `json:"services"`
	Ports      []string              `json:"ports"`
}

type AdminDeployConfig struct {
	*AdminService
}

func (s *AdminDeployConfig) AccessControl() int32 {
	return core.SUDO_ACCESS_CONTROL
}

func (s *AdminDeployConfig) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req deployConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid request body"}))
		return
	}
	if req.AppName == "" || req.Platform == "" || req.DeployRepo == "" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "appName, platform and deployRepo are required"}))
		return
	}

	gitKey, err := s.Cluster().AuthKey("git")
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "git auth key: " + err.Error()}))
		return
	}

	gc := util.GitClient{
		PrivateKey:  gitKey.Git.Key,
		AuthorName:  "tarantula-admin",
		AuthorEmail: "admin@gameclustering.com",
	}
	url := fmt.Sprintf("git@github.com:%s/%s.git", gitKey.Git.Org, req.DeployRepo)
	repoPath := filepath.Join("work", req.DeployRepo)
	if err := gc.CloneOrUpdate(url, repoPath); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "clone deploy repo: " + err.Error()}))
		return
	}

	services := make([]map[string]string, 0, len(req.Services))
	for _, svc := range req.Services {
		m := map[string]string{"name": svc.Name, "network": svc.Network}
		if svc.HttpBinding != "" {
			m["httpBinding"] = svc.HttpBinding
		}
		services = append(services, m)
	}

	defaultDeploy := map[string]any{
		"prefix":      req.Prefix,
		"description": fmt.Sprintf("deploy %s", req.AppName),
	}
	if len(services) > 0 {
		defaultDeploy["services"] = services
	}
	if len(req.Ports) > 0 {
		defaultDeploy["ports"] = req.Ports
	}

	config := map[string]any{
		"default": map[string]any{
			"build": map[string]any{
				"description": fmt.Sprintf("build %s Docker image", req.AppName),
			},
			"deploy": defaultDeploy,
		},
	}

	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "marshal config: " + err.Error()}))
		return
	}

	configDir := filepath.Join(repoPath, req.Platform, req.AppName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "mkdir: " + err.Error()}))
		return
	}
	configPath := filepath.Join(configDir, "deploy.json")
	relPath := filepath.Join(req.Platform, req.AppName, "deploy.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "write file: " + err.Error()}))
		return
	}

	if err := gc.Add(relPath); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "git add: " + err.Error()}))
		return
	}
	commitMsg := fmt.Sprintf("feat: add deploy config for %s/%s", req.Platform, req.AppName)
	if err := gc.Commit(commitMsg); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "git commit: " + err.Error()}))
		return
	}
	if err := gc.Push(); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "git push: " + err.Error()}))
		return
	}

	w.Write(util.ToJson(map[string]any{
		"successful": true,
		"path":       relPath,
		"message":    fmt.Sprintf("deploy.json created at %s in %s", relPath, req.DeployRepo),
	}))
}
