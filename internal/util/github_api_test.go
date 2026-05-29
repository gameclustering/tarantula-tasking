package util

import (
	"fmt"
	"testing"
)

func TestGitHubClient(t *testing.T) {
	vc := vaultClient(t)
	if err := vc.Auth(); err != nil {
		t.Fatalf("vault auth: %s", err)
	}
	ak, err := vc.Load("dev/presence", "git")
	if err != nil {
		t.Fatalf("vault load git: %s", err)
	}
	gh := GitHubApi{Token: ak.Git.Token, Org: ak.Git.Org}
	repos, err := gh.ListRepos()
	if err != nil {
		t.Fatalf("list repos: %s", err)
	}
	fmt.Printf("repos %v\n", repos)
}
