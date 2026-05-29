package util

import (
	"fmt"
	"os"
	"testing"
)

func gitClient(t *testing.T) (*GitClient, func()) {
	vc := vaultClient(t)
	if err := vc.Auth(); err != nil {
		t.Fatalf("vault auth: %s", err)
	}
	ak, err := vc.Load("dev/presence", "git")
	if err != nil {
		t.Fatalf("vault load git: %s", err)
	}
	gc := &GitClient{
		PrivateKey:  ak.Git.Key,
		AuthorName:  ak.Git.User,
		AuthorEmail: ak.Git.Email,
	}
	cleanup := func() {}
	return gc, cleanup
}

func TestGitClientOpen(t *testing.T) {
	gc, cleanup := gitClient(t)
	defer cleanup()

	repoPath, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %s", err)
	}
	// walk up to the repo root (internal/util → repo root)
	gc.Path = repoPath + "/../.."

	if err := gc.Open(); err != nil {
		t.Fatalf("Open: %s", err)
	}

	branch, err := gc.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %s", err)
	}
	fmt.Printf("current branch: %s\n", branch)

	status, err := gc.Status()
	if err != nil {
		t.Fatalf("Status: %s", err)
	}
	fmt.Printf("status:\n%s\n", status)
}

func TestGitClientClone(t *testing.T) {
	gc, cleanup := gitClient(t)
	defer cleanup()

	vc := vaultClient(t)
	if err := vc.Auth(); err != nil {
		t.Fatalf("vault auth: %s", err)
	}
	ak, err := vc.Load("dev/presence", "git")
	if err != nil {
		t.Fatalf("vault load git: %s", err)
	}


	dir, err := os.MkdirTemp("", "git-clone-test-*")
	if err != nil {
		t.Fatalf("mkdirtemp: %s", err)
	}
	defer os.RemoveAll(dir)

	url := fmt.Sprintf("git@github.com:%s/tarantula-protocol.git", ak.Git.Org)
	if err := gc.Clone(url, dir); err != nil {
		t.Fatalf("Clone: %s", err)
	}

	branch, err := gc.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch after clone: %s", err)
	}
	fmt.Printf("cloned branch: %s\n", branch)

	if err := gc.Checkout("dev"); err != nil {
		t.Fatalf("Checkout dev: %s", err)
	}
	fmt.Println("checked out dev")
}
