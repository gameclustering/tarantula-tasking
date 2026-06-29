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
	Name      string            `json:"name"`
	Network   string            `json:"network"`
	HttpBinding string          `json:"httpBinding"`
}

type deployConfigRequest struct {
	AppName     string                 `json:"appName"`
	Platform    string                 `json:"platform"`
	DeployRepo  string                 `json:"deployRepo"`
	Prefix      string                 `json:"prefix"`
	SshUser     string                 `json:"sshUser"`
	VaultHost   string                 `json:"vaultHost"`
	BuildHost   string                 `json:"buildHost"`
	BuildHosts  []string               `json:"buildHosts"`
	Services    []deployConfigService  `json:"services"`
	Ports       []string               `json:"ports"`
	EnvCounts   map[string]int         `json:"envCounts"`
	Settings    map[string]string      `json:"settings"`
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
	if req.SshUser == "" {
		req.SshUser = "tarantula"
	}
	if req.VaultHost == "" {
		req.VaultHost = "https://vault.gameclustering.com"
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

	// Build deploy.json structure.
	services := make([]map[string]string, 0, len(req.Services))
	for _, svc := range req.Services {
		m := map[string]string{"name": svc.Name, "network": svc.Network}
		if svc.HttpBinding != "" {
			m["httpBinding"] = svc.HttpBinding
		}
		services = append(services, m)
	}

	defaultSettings := map[string]string{}
	for k, v := range req.Settings {
		defaultSettings[k] = v
	}

	defaultDeploy := map[string]any{
		"prefix":        req.Prefix,
		"instanceNumber": 1,
		"sshUser":       req.SshUser,
		"vaultHost":     req.VaultHost,
		"description":   fmt.Sprintf("deploy %s", req.AppName),
		"settings":      defaultSettings,
	}
	if len(services) > 0 {
		defaultDeploy["services"] = services
	}
	if len(req.Ports) > 0 {
		defaultDeploy["ports"] = req.Ports
	}

	defaultBuild := map[string]any{
		"sshUser":     req.SshUser,
		"description": fmt.Sprintf("build %s Docker image", req.AppName),
	}
	if req.BuildHost != "" {
		defaultBuild["buildHost"] = req.BuildHost
	}
	if len(req.BuildHosts) > 0 {
		defaultBuild["buildHosts"] = req.BuildHosts
	}

	config := map[string]any{
		"default": map[string]any{
			"build":  defaultBuild,
			"deploy": defaultDeploy,
		},
	}
	for env, count := range req.EnvCounts {
		if env == "default" || count == 0 {
			continue
		}
		config[env] = map[string]any{
			"deploy": map[string]any{"instanceNumber": count},
		}
	}

	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "marshal config: " + err.Error()}))
		return
	}

	// Write file to repo.
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
