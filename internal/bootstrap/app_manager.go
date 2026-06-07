package bootstrap

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"github.com/rs/zerolog"
)

type AppManager struct {
	Auth   core.Authenticator
	CACert []byte // PEM-encoded CA cert; used by gRPC client to verify the postoffice server
	Sql    persistence.Postgresql
	F      core.Env
	seq    core.Sequence

	cluster *ClusterManager

	log       io.Writer //zerolog.Logger
	forward   LogForwarder
	threshold zerolog.Level
}

func (s *AppManager) Authenticator() core.Authenticator {
	return s.Auth
}
func (s *AppManager) Sequence() core.Sequence {
	return s.seq
}

func (c *AppManager) Cluster() core.ClusterService {
	return c.cluster
}

func (s *AppManager) NodeId() string {
	return s.F.NodeName
}

func (s *AppManager) ClusterMember() bool {
	return s.F.IsClusterMember
}

func (s *AppManager) RegisterLogForwarder(threshold zerolog.Level, logf LogForwarder) {
	s.threshold = threshold
	s.forward = logf
}
func (s *AppManager) Start(f core.Env) error {
	s.F = f
	s.initLogger(f)
	sfk := util.NewSnowflake(f.NodeId, util.EpochMillisecondsFromMidnight(2020, 1, 1))
	s.seq = &sfk
	if f.IsClusterMember {
		return nil
	}
	retries := 10
	for {
		err := s.loadAuthContext()
		if err == nil {
			break
		}
		retries--
		if retries == 0 {
			panic(err)
		}
		time.Sleep(3 * time.Second)
		core.AppLog.Warn().Msgf("load client credentials from %s retries remaining %d", f.Vlt.Host, retries)
	}
	core.AppLog.Warn().Msgf("Starting cluster client to %s", f.PostOfficeHost)
	s.cluster = &ClusterManager{App: s}
	s.RegisterLogForwarder(zerolog.ErrorLevel, s.cluster)

	err := s.cluster.connect(f.PostOfficeHost)
	if err != nil {
		panic(err.Error())
	}
	if f.SqlEnabled {
		key, err := s.cluster.AuthKey("sql")
		if err != nil {
			panic(err)
		}
		//postgres://[user]:[password]@[host]:5432
		user := key.Sql.User
		pwd := key.Sql.Password
		hst := key.Sql.Host
		pgs := fmt.Sprintf("postgres://%s:%s@%s:5432", user, pwd, hst)
		core.AppLog.Info().Msgf("connecting sql %s", hst)
		dbCreate := persistence.Postgresql{Url: pgs + "/postgres"}
		err = dbCreate.CreateDatabase(fmt.Sprintf("CREATE DATABASE %s_%s_%s", f.Prefix, "tarantula", f.GroupName))
		if err != nil {
			core.AppLog.Warn().Msgf("failed to create database %s", err.Error())
		}
		sql := persistence.Postgresql{Url: pgs + "/" + f.Prefix + "_tarantula_" + f.GroupName}
		err = sql.Create()
		if err != nil {
			panic(err)
		}
		s.Sql = sql
	}
	return nil
}

func (s *AppManager) Shutdown() {
	if !s.F.IsClusterMember {
		s.cluster.disconnect()
	}
	s.Sql.Close()
	core.AppLog.Info().Msg("app manager shutting down ...")
}

func (s *AppManager) Context() string {
	return s.F.GroupName
}

func (s *AppManager) Service() TarantulaService {
	return s
}

func (s *AppManager) LoadAuth(ak *protocol.AuthKey) (core.Authenticator, error) {
	tkn := util.JwtHMac{Alg: core.JWT_ALG, Ksz: core.JWT_KEY_SIZE}
	ci := util.Aes{Ksz: core.CIPHER_KEY_SIZE}
	jk, err := util.KeyFromBase64(string(ak.Jwt))
	if err != nil {
		return nil, err
	}
	tkn.HMacFromKey(jk)
	ck, err := util.KeyFromBase64(string(ak.Cipher))
	if err != nil {
		return nil, err
	}
	ci.AesGcmFromKey(ck)
	return &AuthManager{Tkn: &tkn, Cipher: &ci, Kid: "presence"}, nil
}

func (c *AppManager) Write(data []byte) (int, error) {
	return c.log.Write(data)
}

func (c *AppManager) WriteLevel(level zerolog.Level, data []byte) (int, error) {
	if c.forward != nil && level >= c.threshold {
		cp := append([]byte{}, data...)
		c.forward.Forward(level, cp)
	}
	return c.Write(data)
}

func (c *AppManager) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	_, f, line, ok := runtime.Caller(3)
	if !ok {
		e.Str("source", "unknown")
		return
	}
	e.Str("source", fmt.Sprintf("%s:%d", f, line))
}

func (c *AppManager) initLogger(f core.Env) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zerolog.TimeFieldFormat = time.RFC3339
	if f.Standalone {
		CreateTestLog()
		return
	}
	err := os.MkdirAll(f.LogDir+"/log", 0755)
	if err != nil {
		CreateTestLog()
		return
	}
	opt := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	if f.LogTruncated {
		opt = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	file, err := os.OpenFile(f.LogDir+"/log/tarantula.log", opt, 0644)
	if err != nil {
		CreateTestLog()
		return
	}
	c.log = file
	core.AppLog = zerolog.New(zerolog.MultiLevelWriter(c)).With().Timestamp().Logger().Hook(c)
	core.AppLog.Info().Msg("Initialized app log")

}

func (c *AppManager) ToBytes(systemId uint64) []byte {
	buff := core.NewBuffer(8)
	buff.WriteUInt64(systemId)
	buff.Flip()
	key, _ := buff.Read(0)
	return key
}

func (c *AppManager) ToSystemId(key []byte) int64 {
	buff := core.NewBuffer(8)
	buff.Write(key)
	buff.Flip()
	sysId, _ := buff.ReadUInt64()
	return int64(sysId)
}

func (c *AppManager) loadAuthContext() error {
	vault := util.VaultClient{Host: c.F.Vlt.Host, Token: c.F.Vlt.Token}
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
	c.CACert = []byte(auth.Cert)
	return nil //os.WriteFile(core.CERT_NAME, []byte(auth.Cert), 0600)
}
