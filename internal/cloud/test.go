package cloud

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewTest(mgr *bootstrap.AppManager, store *Store, vaultKey string, factory PlatformFactory) *protocol.TccTransationListener {
	h := &testHandler{mgr: mgr, store: store, vaultKey: vaultKey, factory: factory}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = h.reserve
	tcc.Confirm = h.confirm
	tcc.Cancel = h.cancel
	return &tcc
}

type testHandler struct {
	mgr      *bootstrap.AppManager
	store    *Store
	vaultKey string
	factory  PlatformFactory
}

func (h *testHandler) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("test reserve %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	gitKey, err := h.mgr.Cluster().AuthKey("git")
	if err != nil {
		return fmt.Errorf("git auth key: %w", err)
	}
	planName := plan.Name
	if planName == "" && plan.AppRepo != nil {
		planName = plan.AppRepo.Name
	}
	cfg, err := LoadDeployConfig(plan.DeployRepo, plan.Platform, planName, gitKey)
	if err != nil {
		return fmt.Errorf("deploy config: %w", err)
	}
	testPhase := cfg.Resolve(plan.Env, "test")
	if testPhase.TestRepo == "" || testPhase.Prefix == "" {
		core.AppLog.Info().Msgf("test [%s]: no test config, skipping", planName)
		return h.store.Insert(t.Meta)
	}
	deployPhase := cfg.Resolve(plan.Env, "deploy")

	platformKey, err := h.mgr.Cluster().AuthKey(h.vaultKey)
	if err != nil {
		return fmt.Errorf("%s auth key: %w", h.vaultKey, err)
	}
	platform, err := h.factory(testPhase, platformKey)
	if err != nil {
		return fmt.Errorf("platform init: %w", err)
	}
	defer platform.Close()

	seq := int(plan.Seq)
	if seq < 1 {
		seq = 1
	}
	testName := fmt.Sprintf("%s-%02d", testPhase.Prefix, seq)
	if err := platform.Provision(testName); err != nil {
		return fmt.Errorf("provision %s: %w", testName, err)
	}
	defer func() {
		if err := platform.Remove(testName); err != nil {
			core.AppLog.Warn().Msgf("test: remove %s: %s", testName, err)
		}
	}()
	core.AppLog.Info().Msgf("test: instance %s provisioned", testName)

	testIP, err := platform.IP(testName)
	if err != nil {
		return fmt.Errorf("test VM IP: %w", err)
	}
	sshUser := testPhase.SshUser
	if sshUser == "" {
		sshUser = platform.SSHUser()
	}
	ssh := util.SshClient{Host: testIP, User: sshUser, PrivateKey: platform.SSHKey(), KHFile: "../.ssh/known_hosts"}
	deadline := time.Now().Add(5 * time.Minute)
	for {
		if err := ssh.WithKey(); err == nil {
			break
		} else if time.Now().After(deadline) {
			return fmt.Errorf("ssh to %s: timed out", testName)
		}
		core.AppLog.Debug().Msgf("test [%s]: waiting for SSH...", testName)
		time.Sleep(10 * time.Second)
	}
	defer ssh.Close()

	if err := h.installK6(ssh, testName); err != nil {
		return fmt.Errorf("install k6 on %s: %w", testName, err)
	}

	keyFile, err := os.CreateTemp("", "id_ed25519_*")
	if err != nil {
		return fmt.Errorf("create temp key file: %w", err)
	}
	defer os.Remove(keyFile.Name())
	if _, err := keyFile.WriteString(util.NormalizePemKey(gitKey.Git.Key)); err != nil {
		keyFile.Close()
		return fmt.Errorf("write git key: %w", err)
	}
	keyFile.Close()
	if err := h.uploadGitKey(ssh, sshUser, keyFile.Name(), testName); err != nil {
		return fmt.Errorf("upload git key: %w", err)
	}

	appPrefix := testPhase.AppPrefix
	if appPrefix == "" {
		appPrefix = deployPhase.Prefix
	}
	appName := fmt.Sprintf("%s-%02d", appPrefix, seq)
	appIP, err := platform.IP(appName)
	if err != nil {
		return fmt.Errorf("app instance IP (%s): %w", appName, err)
	}
	baseURL := fmt.Sprintf("http://%s:8080", appIP)
	core.AppLog.Info().Msgf("test: targeting %s at %s", appName, baseURL)

	cloneURL := fmt.Sprintf("git@github.com:%s/%s.git", gitKey.Git.Org, testPhase.TestRepo)
	var out bytes.Buffer
	for _, cmd := range []string{
		"mkdir -p ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts 2>/dev/null",
		fmt.Sprintf("git clone %s tests", cloneURL),
	} {
		out.Reset()
		ctx2m, cancel2m := context.WithTimeout(context.Background(), 2*time.Minute)
		err := ssh.Run(ctx2m, cmd, &out)
		cancel2m()
		if err != nil {
			core.AppLog.Warn().Msgf("test [%s]: clone failed: %s — %s", testName, err, out.String())
			return fmt.Errorf("clone test repo: %w — %s", err, out.String())
		}
	}

	// Compute the promotion tag now so we can pass it to entrypoint as APP_TAG
	reportTag := plan.Env
	if p := testPhase.Promotion; p != nil && p.TagPattern != "" {
		reportTag = fmt.Sprintf(p.TagPattern, plan.Env)
	}

	out.Reset()
	ctx30m, cancel30m := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel30m()
	if err := ssh.Run(ctx30m, fmt.Sprintf("BASE_URL='%s' APP_TAG='%s' bash tests/entrypoint.sh 2>&1", baseURL, reportTag), &out); err != nil {
		core.AppLog.Warn().Msgf("test [%s]: entrypoint failed: %s\n%s", testName, err, strings.TrimSpace(out.String()))
		return fmt.Errorf("tests failed: %w — %s", err, strings.TrimSpace(out.String()))
	}
	core.AppLog.Info().Msgf("test [%s]: all tests passed", planName)

	// Only Seq=1 promotes — prevents N workers racing to push the same git tag.
	if seq == 1 {
		if p := testPhase.Promotion; p != nil && p.Repo != "" && p.TagPattern != "" {
			tag := fmt.Sprintf(p.TagPattern, plan.Env)
			if err := h.pushTag(ssh, gitKey.Git.Org, p.Repo, tag); err != nil {
				core.AppLog.Warn().Msgf("test: push promotion tag %s: %s", tag, err)
			} else {
				core.AppLog.Info().Msgf("test: pushed promotion tag %s → %s", tag, p.Repo)
			}
		}
	}

	return h.store.Insert(t.Meta)
}

func (h *testHandler) installK6(ssh util.SshClient, name string) error {
	var out bytes.Buffer
	cmds := []string{
		"sudo apt-get update -qq",
		"sudo apt-get install -y -qq ca-certificates gnupg curl git",
		"sudo mkdir -p /etc/apt/keyrings",
		"curl -fsSL --retry 3 --retry-delay 2 https://dl.k6.io/key.gpg -o /tmp/k6.gpg.asc",
		"sudo gpg --dearmor -o /etc/apt/keyrings/k6-archive-keyring.gpg /tmp/k6.gpg.asc",
		`echo "deb [signed-by=/etc/apt/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list`,
		"sudo apt-get update -qq",
		"sudo apt-get install -y -qq k6",
	}
	for _, cmd := range cmds {
		out.Reset()
		ctx10m, cancel10m := context.WithTimeout(context.Background(), 10*time.Minute)
		err := ssh.Run(ctx10m, cmd, &out)
		cancel10m()
		if err != nil {
			return fmt.Errorf("cmd %q: %w — %s", cmd, err, out.String())
		}
		core.AppLog.Debug().Msgf("test [%s]: %s", name, strings.TrimSpace(out.String()))
	}
	return nil
}

func (h *testHandler) uploadGitKey(ssh util.SshClient, user, keyFile, name string) error {
	f, err := os.Open(keyFile)
	if err != nil {
		return err
	}
	defer f.Close()
	remotePath := fmt.Sprintf("/home/%s/.ssh/id_ed25519", user)
	ctx2m, cancel2m := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel2m()
	if err := ssh.Upload(ctx2m, f, remotePath, "0600"); err != nil {
		return err
	}
	core.AppLog.Info().Msgf("test: git key uploaded to %s on %s", remotePath, name)
	return nil
}

func (h *testHandler) pushTag(ssh util.SshClient, org, repo, tag string) error {
	cloneURL := fmt.Sprintf("git@github.com:%s/%s.git", org, repo)
	var out bytes.Buffer
	for _, cmd := range []string{
		fmt.Sprintf("git clone --depth 1 %s promo", cloneURL),
		fmt.Sprintf("git -C promo tag %s", tag),
		fmt.Sprintf("git -C promo push origin %s", tag),
	} {
		out.Reset()
		ctx2m, cancel2m := context.WithTimeout(context.Background(), 2*time.Minute)
		err := ssh.Run(ctx2m, cmd, &out)
		cancel2m()
		if err != nil {
			return fmt.Errorf("cmd %q: %w — %s", cmd, err, out.String())
		}
	}
	return nil
}

func (h *testHandler) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("test confirm %v", t.Meta)
	return h.store.Insert(t.Meta)
}

func (h *testHandler) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("test cancel %v", t.Meta)
	return h.store.Insert(t.Meta)
}
