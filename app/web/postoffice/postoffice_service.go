package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
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
	s.AppManager.Start(env)
	vault := util.VaultClient{Host: s.F.Vlt.Host, Token: s.F.Vlt.Token}
	retries := 10
	for {
		err := s.loadAuthContext(&vault)
		if err == nil {
			break
		}
		retries--
		if retries == 0 {
			panic(err)
		}
		time.Sleep(3 * time.Second)
		core.AppLog.Warn().Msgf("load credentials from %s retries remaining %d", vault.Host, retries)
	}
	seeds := resolveSeeds(env.ClusterBootstrap)
	m := clustering.MemberlistManager{StoreDir: fmt.Sprintf("%s/%s", env.HomeDir, env.GroupName), Ctx: s.F.PresenceCtx()}
	m.Seed = seeds
	m.Binding = env.NodeName
	err := m.Start(fmt.Appendf([]byte{}, "%s:%s", s.Context(), s.NodeId()), s.Authenticator(), s.Sequence(), &vault)
	if err != nil {
		core.AppLog.Warn().Msgf("no cluster can join %s", err.Error())
		return err
	}
	s.mm = &m
	s.mm.DWait.Wait()
	s.started = true
	http.HandleFunc("/postoffice/seeds", s.seedsHandler)
	core.AppLog.Info().Msgf("postoffice service started %s %s", env.HttpBinding, env.HomeDir)
	return nil
}

func (s *PostofficeService) seedsHandler(w http.ResponseWriter, r *http.Request) {
	members := s.mm.Members()
	seeds := make([]string, 0, len(members))
	for _, m := range members {
		host, _, err := net.SplitHostPort(m.Address())
		if err != nil {
			continue
		}
		seeds = append(seeds, host)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(seeds)
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
	return os.WriteFile(core.CERT_NAME, []byte(auth.Cert), 0600)
}

// resolveSeeds queries the bootstrap address for current cluster members.
// Returns nil if bootstrap is empty or unreachable — the node starts as the first member.
func resolveSeeds(bootstrap string) []string {
	if bootstrap == "" {
		return nil
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(strings.TrimRight(bootstrap, "/") + "/postoffice/seeds")
	if err != nil {
		core.AppLog.Info().Msgf("cluster bootstrap unreachable at %s, starting as first node: %s", bootstrap, err.Error())
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		core.AppLog.Info().Msgf("cluster bootstrap %s returned %d, starting as first node", bootstrap, resp.StatusCode)
		return nil
	}
	var seeds []string
	if err := json.NewDecoder(resp.Body).Decode(&seeds); err != nil {
		core.AppLog.Warn().Msgf("failed to decode seeds from %s: %s", bootstrap, err.Error())
		return nil
	}
	core.AppLog.Info().Msgf("resolved %d cluster seeds from %s: %v", len(seeds), bootstrap, seeds)
	return seeds
}
