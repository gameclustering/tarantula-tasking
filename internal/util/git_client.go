package util

import (
	"fmt"
	"strings"
	"time"

	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/object"
	gossh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

type GitClient struct {
	Path        string
	PrivateKey  string // PEM-encoded SSH private key for remote operations
	AuthorName  string
	AuthorEmail string
	repo        *git.Repository
}

func (g *GitClient) Open() error {
	r, err := git.PlainOpen(g.Path)
	if err != nil {
		return err
	}
	g.repo = r
	return nil
}

func (g *GitClient) sshOpt() (client.Option, error) {
	key := normalizePemKey(g.PrivateKey)
	auth, err := gossh.NewPublicKeys("git", []byte(key), "")
	if err != nil {
		return nil, err
	}
	return client.WithSSHAuth(auth), nil
}

// normalizePemKey ensures the PEM footer has the required 5 trailing dashes
// and ends with a newline. Vault occasionally trims the last character.
func normalizePemKey(key string) string {
	key = strings.TrimRight(key, "\n")
	if !strings.HasSuffix(key, "-----") {
		key += "-"
	}
	return key + "\n"
}

func (g *GitClient) CurrentBranch() (string, error) {
	head, err := g.repo.Head()
	if err != nil {
		return "", err
	}
	return head.Name().Short(), nil
}

func (g *GitClient) Add(path string) error {
	w, err := g.repo.Worktree()
	if err != nil {
		return err
	}
	_, err = w.Add(path)
	return err
}

func (g *GitClient) Remove(path string) error {
	w, err := g.repo.Worktree()
	if err != nil {
		return err
	}
	_, err = w.Remove(path)
	return err
}

func (g *GitClient) Commit(msg string) error {
	w, err := g.repo.Worktree()
	if err != nil {
		return err
	}
	opts := &git.CommitOptions{}
	if g.AuthorName != "" {
		opts.Author = &object.Signature{
			Name:  g.AuthorName,
			Email: g.AuthorEmail,
			When:  time.Now(),
		}
	}
	_, err = w.Commit(msg, opts)
	return err
}

func (g *GitClient) Push() error {
	opt, err := g.sshOpt()
	if err != nil {
		return err
	}
	err = g.repo.Push(&git.PushOptions{ClientOptions: []client.Option{opt}})
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

func (g *GitClient) Pull() error {
	w, err := g.repo.Worktree()
	if err != nil {
		return err
	}
	opt, err := g.sshOpt()
	if err != nil {
		return err
	}
	err = w.Pull(&git.PullOptions{ClientOptions: []client.Option{opt}})
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

func (g *GitClient) Status() (string, error) {
	w, err := g.repo.Worktree()
	if err != nil {
		return "", err
	}
	st, err := w.Status()
	if err != nil {
		return "", err
	}
	return st.String(), nil
}

func (g *GitClient) Checkout(branch string) error {
	w, err := g.repo.Worktree()
	if err != nil {
		return err
	}
	localRef := plumbing.NewBranchReferenceName(branch)
	err = w.Checkout(&git.CheckoutOptions{Branch: localRef})
	if err == nil {
		return nil
	}
	// local branch not found — create it from the remote tracking ref
	remoteRef := plumbing.NewRemoteReferenceName("origin", branch)
	ref, err2 := g.repo.Reference(remoteRef, true)
	if err2 != nil {
		return err // return original error
	}
	return w.Checkout(&git.CheckoutOptions{
		Branch: localRef,
		Hash:   ref.Hash(),
		Create: true,
	})
}

func (g *GitClient) Clone(url, path string) error {
	opt, err := g.sshOpt()
	if err != nil {
		return err
	}
	r, err := git.PlainClone(path, &git.CloneOptions{
		URL:           url,
		ClientOptions: []client.Option{opt},
	})
	if err != nil {
		return err
	}
	g.repo = r
	g.Path = path
	return nil
}

func (g *GitClient) CheckoutTag(tag string) error {
	w, err := g.repo.Worktree()
	if err != nil {
		return err
	}
	ref, err := g.repo.Tag(tag)
	if err != nil {
		return fmt.Errorf("tag %s not found: %w", tag, err)
	}
	return w.Checkout(&git.CheckoutOptions{Hash: ref.Hash()})
}
