package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
	"gameclustering.com/postoffice/clustering"
)

type PostofficeService struct {
	bootstrap.AppManager
	mm      *clustering.MemberlistManager
	started bool
}

func (s *PostofficeService) Config() string {
	return "/etc/tarantula/postoffice-conf.json"
}

func (s *PostofficeService) Start(env core.Env) error {
	env.AuthLevel = core.ADMIN_ACCESS_CONTROL
	env.IsClusterMember = true
	err := s.AppManager.Start(env)
	if err != nil {
		return err
	}
	vault := util.VaultClient{Host: s.F.Vlt.Host, Token: s.F.Vlt.Token}
	retries := 10
	for {
		err := s.loadAuthContext(&vault)
		if err == nil {
			break
		}
		retries--
		if retries == 0 {
			return err
		}
		time.Sleep(3 * time.Second)
		core.AppLog.Warn().Msgf("load credentials from %s retries remaining %d with %s", vault.Host, retries, err.Error())
	}
	seeds := resolveSeeds(env.ClusterBootstrap)
	m := clustering.MemberlistManager{StoreDir: fmt.Sprintf("%s/%s", env.HomeDir, env.GroupName), Ctx: s.F.PresenceCtx()}
	m.Seed = seeds
	m.Binding = env.NodeName
	m.AdvertiseAddr = env.PostOfficeAdvertiseIP
	m.LocalHost = env.PostOfficeHost
	err = m.Start(fmt.Appendf([]byte{}, "%s:%s", s.Context(), s.NodeId()), s.Authenticator(), s.Sequence(), &vault)
	if err != nil {
		core.AppLog.Warn().Msgf("no cluster can join %s", err.Error())
		return err
	}
	s.mm = &m
	s.started = true
	http.HandleFunc("/postoffice/seeds", bootstrap.Logging(&ClusterSeedGet{s}))
	http.HandleFunc("/postoffice/health", bootstrap.Logging(&ClusterHealthCheck{s}))
	
	registerServiceProxy("/admin/", os.Getenv("ADMIN_HOST"), "http://admin:8080")
	registerServiceProxy("/cloud/meta/task/", os.Getenv("GOOGLE_CLOUD_HOST"), "http://google_cloud:8080")
	core.AppLog.Info().Msgf("postoffice service started %s %s", env.HttpBinding, env.HomeDir)
	return nil
}

// registerServiceProxy forwards /prefix/* requests to the target service.
// envOverride allows the target to be overridden via env var for testing.
func registerServiceProxy(prefix, envOverride, defaultTarget string) {
	target := defaultTarget
	if envOverride != "" {
		target = envOverride
	}
	u, err := url.Parse(target)
	if err != nil {
		core.AppLog.Warn().Msgf("invalid proxy target %s for %s: %s", target, prefix, err)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	http.Handle(prefix, proxy)
	core.AppLog.Info().Msgf("proxying %s* → %s", prefix, target)
}

func (s *PostofficeService) Shutdown() {
	s.started = false
	core.AppLog.Info().Msg("postoffice service shutting down ...")
	s.AppManager.Shutdown()
	s.mm.ShutdownHook()
}

func (c *PostofficeService) loadAuthContext(vault *util.VaultClient) error {
	err := vault.Auth()
	if err != nil {
		return err
	}
	auth, err := vault.Load(c.F.PresenceCtx(), "auth")
	if err != nil {
		return err
	}
	au, err := c.LoadAuth(auth)
	if err != nil {
		return err
	}
	c.Auth = au
	return nil
}

// resolveSeeds queries the bootstrap address for current cluster members.
// Returns nil if bootstrap is empty or unreachable — the node starts as the first member.
// Retries up to 5 times with 3s delay to handle cases where the seed node is still starting.
func resolveSeeds(bootstrap string) []string {
	if bootstrap == "" {
		return nil
	}
	url := strings.TrimRight(bootstrap, "/") + "/postoffice/seeds"
	client := &http.Client{Timeout: 5 * time.Second}
	for attempt := 1; attempt <= 5; attempt++ {
		resp, err := client.Get(url)
		if err != nil {
			core.AppLog.Info().Msgf("cluster bootstrap unreachable at %s (attempt %d/5): %s", bootstrap, attempt, err.Error())
		} else {
			if resp.StatusCode == http.StatusOK {
				var seeds []string
				decErr := json.NewDecoder(resp.Body).Decode(&seeds)
				resp.Body.Close()
				if decErr != nil {
					core.AppLog.Warn().Msgf("failed to decode seeds from %s: %s", bootstrap, decErr.Error())
					return nil
				}
				core.AppLog.Info().Msgf("resolved %d cluster seeds from %s: %v", len(seeds), bootstrap, seeds)
				return seeds
			}
			resp.Body.Close()
			core.AppLog.Info().Msgf("cluster bootstrap %s returned %d (attempt %d/5), retrying...", bootstrap, resp.StatusCode, attempt)
		}
		if attempt < 5 {
			time.Sleep(3 * time.Second)
		}
	}
	core.AppLog.Warn().Msgf("cluster bootstrap %s failed after 5 attempts, starting as first node", bootstrap)
	return nil
}
