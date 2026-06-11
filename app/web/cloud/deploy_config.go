package main

import (
	"fmt"
	"path/filepath"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
)

// loadDeployConfig clones (or updates) the deploy repo, checks out the
// specified ref, and returns the platform-agnostic deploy config for the
// given vendor. The deploy repo is expected to have a file at
// <vendor>/deploy.json (e.g. gcp/deploy.json, vultr/deploy.json).
func loadDeployConfig(deployRepo *protocol.RepoObject, vendor string, gitKey *protocol.AuthKey) (*core.DeployConfig, error) {
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
	cfg, err := core.LoadDeployConfig(filepath.Join(repoPath, vendor, "deploy.json"))
	if err != nil {
		return nil, fmt.Errorf("load %s deploy config: %w", vendor, err)
	}
	return cfg, nil
}

