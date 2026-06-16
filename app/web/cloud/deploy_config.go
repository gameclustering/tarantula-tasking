package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
)

// loadDeployConfig clones (or updates) the deploy repo, checks out the
// specified ref, and returns the deploy config for the given platform.
// It looks for <platform>/<name>/deploy.json when name is non-empty,
// falling back to <platform>/deploy.json.
func loadDeployConfig(deployRepo *protocol.RepoObject, platform, name string, gitKey *protocol.AuthKey) (*core.DeployConfig, error) {
	if deployRepo == nil || deployRepo.Name == "" {
		return nil, fmt.Errorf("deploy repo is required")
	}
	gc := util.GitClient{PrivateKey: gitKey.Git.Key}
	url := fmt.Sprintf("git@github.com:%s/%s.git", gitKey.Git.Org, deployRepo.Name)
	repoPath := filepath.Join("work", deployRepo.Name)
	if err := gc.CloneOrUpdate(url, repoPath); err != nil {
		return nil, fmt.Errorf("clone/update %s: %w", deployRepo.Name, err)
	}
	if err := gc.CheckoutRef(deployRepo.Tag, deployRepo.Branch); err != nil {
		return nil, fmt.Errorf("checkout ref: %w", err)
	}
	configPath := filepath.Join(repoPath, platform, "deploy.json")
	if name != "" {
		sub := filepath.Join(repoPath, platform, name, "deploy.json")
		if _, err := os.Stat(sub); err == nil {
			configPath = sub
		}
	}
	cfg, err := core.LoadDeployConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load %s/%s deploy config: %w", platform, name, err)
	}
	return cfg, nil
}
