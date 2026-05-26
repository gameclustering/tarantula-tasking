package main

import (
	"fmt"
	"os"
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
	return "/postoffice-conf.json"
	//return "/etc/tarantula/postoffice-conf.json"
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
	m := clustering.MemberlistManager{StoreDir: fmt.Sprintf("%s/%s", env.HomeDir, env.GroupName), Ctx: s.F.PresenceCtx()}
	m.Seed = env.ClusterSeed //[]string{"192.168.1.11", "192.168.1.3"}
	m.Binding = env.NodeName
	err := m.Start(fmt.Appendf([]byte{}, "%s:%s", s.Context(), s.NodeId()), s.Authenticator(), s.Sequence(), &vault)
	if err != nil {
		core.AppLog.Warn().Msgf("no cluster can join %s", err.Error())
		return err
	}
	s.mm = &m
	s.mm.DWait.Wait()
	s.started = true
	core.AppLog.Info().Msgf("postoffice service started %s %s", env.HttpBinding, env.HomeDir)
	return nil
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
	err = os.WriteFile(clustering.KEY_NAME, []byte(auth.Key), 0600)
	if err != nil {
		return err
	}
	return os.WriteFile(core.CERT_NAME, []byte(auth.Cert), 0600)
}
