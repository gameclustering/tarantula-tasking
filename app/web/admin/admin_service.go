package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/event"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
)

type AdminService struct {
	bootstrap.AppManager
	assetDir   string
	contentDir string
	publishDir string
}

func (s *AdminService) Config() string {
	return "/etc/tarantula/admin-conf.json"
}

func (s *AdminService) Start(f core.Env) error {
	f.AuthLevel = core.ADMIN_ACCESS_CONTROL
	s.AppManager.Start(f)
	s.contentDir = fmt.Sprintf("%s/%s", f.HomeDir, "bin")
	s.assetDir = fmt.Sprintf("%s/%s/%s", f.HomeDir, f.GroupName, "asset")
	os.MkdirAll(s.assetDir, 0755)
	s.publishDir = s.contentDir + "/tarantula"
	err := s.createSchema()
	if err != nil {
		return err
	}
	hash, err := s.Authenticator().HashPassword("password")
	if err != nil {
		return err
	}
	err = s.SaveLogin(&protocol.LoginObject{Name: "root", Password: hash, AccessControl: uint32(core.SUDO_ACCESS_CONTROL)})
	if err != nil {
		core.AppLog.Debug().Msg("Root already existed")
	}
	s.Cluster().Subscribe(event.TASK_TOPIC_NAME, &protocol.TopicEventListener{C: func() proto.Message {
		return &protocol.TaskEvent{}
	}, M: func(m proto.Message) {
		ro, ok := m.(*protocol.TaskEvent)
		if ok {
			core.AppLog.Debug().Msgf("validator event %v", ro.Validator)
			core.AppLog.Debug().Msgf("transaction event %v", ro.Job)
		} else {
			core.AppLog.Debug().Msg("wrong type")
		}
	}})
	http.Handle("/admin/webprotected/{name}", bootstrap.Logging(&AdminWebProtected{AdminService: s}))

	http.Handle("/admin/cs/message/send", bootstrap.Logging(&CSMessager{AdminService: s}))

	http.Handle("/admin/cs/query/topic/{topic}", bootstrap.Logging(&CSQueryTopic{AdminService: s}))
	http.Handle("/admin/cs/query/object/{topic}", bootstrap.Logging(&CSQueryObject{AdminService: s}))

	http.Handle("/admin/login/add", bootstrap.Logging(&SudoAddLogin{AdminService: s}))
	http.Handle("/admin/password", bootstrap.Logging(&AdminChangePwd{AdminService: s}))
	http.Handle("/admin/accesskey", bootstrap.Logging(&AdminCreateAccessKey{AdminService: s}))
	http.Handle("/admin/login", bootstrap.Logging(&AdminLogin{AdminService: s}))

	http.Handle("/admin/presence/hashring", bootstrap.Logging(&AdminHashRingEndpoint{AdminService: s}))
	http.Handle("/admin/presence/keyring/{key}", bootstrap.Logging(&AdminKeyRingEndpoint{AdminService: s}))
	http.Handle("/admin/presence/subscription/task", bootstrap.Logging(&AdminSubscriptionTaskEndpoint{AdminService: s}))
	http.Handle("/admin/presence/subscription/topic", bootstrap.Logging(&AdminSubscriptionTopicEndpoint{AdminService: s}))

	http.Handle("/admin/cluster/create", bootstrap.Logging(&AdminClusterCreate{AdminService: s}))

	http.Handle("GET /admin/cluster/repo", bootstrap.Logging(&AdminClusterRepoList{AdminService: s}))
	http.Handle("POST /admin/cluster/repo", bootstrap.Logging(&AdminClusterRepoCreate{AdminService: s}))
	http.Handle("DELETE /admin/cluster/repo/{id}", bootstrap.Logging(&AdminClusterRepoDelete{AdminService: s}))

	http.Handle("POST /admin/vps/setup", bootstrap.Logging(&AdminVpsSetup{AdminService: s}))

	core.AppLog.Info().Msgf("Admin service started %s\n", f.HttpBinding)
	return nil
}

func (s *AdminService) Shutdown() {
	s.Cluster().Unsubscribe(event.TASK_TOPIC_NAME)
	time.Sleep(1 * time.Second)
	s.AppManager.Shutdown()
}
