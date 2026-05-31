package main

import (
	"fmt"
	"path/filepath"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
)

// loadGcpDeployConfig clones (or updates) the deploy repo, checks out the
// specified tag/branch, and returns the parsed GCP deploy config.
func loadGcpDeployConfig(deployRepo *protocol.RepoObject, gitKey *protocol.AuthKey) (*core.GcpDeployConfig, error) {
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

	cfg, err := core.LoadGcpDeployConfig(filepath.Join(repoPath, "gcp", "deploy.json"))
	if err != nil {
		return nil, fmt.Errorf("load gcp deploy config: %w", err)
	}
	return cfg, nil
}
