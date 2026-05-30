package main

import (
	"net/http"
	"time"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/event"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
)

type CloudService struct {
	bootstrap.AppManager
}

func (s *CloudService) Config() string {
	return "/etc/tarantula/cloud-conf.json"
}

func (s *CloudService) Start(f core.Env) error {
	s.AppManager.Start(f)
	if err := s.createSchema(); err != nil {
		return err
	}
	s.Cluster().Subscribe(event.MESSAGE_TOPIC_NAME, &protocol.TopicEventListener{C: func() proto.Message {
		return &protocol.MessageEvent{}
	}, M: func(m proto.Message) {
		ro, ok := m.(*protocol.MessageEvent)
		if ok {
			core.AppLog.Debug().Msgf("MESSAGE event %s %s", ro.Message, ro.Source)
		} else {
			core.AppLog.Debug().Msg("wrong type")
		}
	}})
	s.Cluster().Register("check", NewRepositoryObejctCheck(s))
	s.Cluster().Register("create", NewVMObjectCreate(s))
	s.Cluster().Register("update", NewVMObjectUpdate(s))
	s.Cluster().Register("build", NewVMObjectBuild(s))
	s.Cluster().Register("deploy", NewVMObjectDeploy(s))

	http.Handle("/cloud/meta/task/{taskId}", bootstrap.Logging(&CloudMetaGet{CloudService: s}))

	core.AppLog.Info().Msgf("Cloud service started %s", f.HttpBinding)
	return nil
}

func (s *CloudService) Shutdown() {
	s.Cluster().Unregister("check")
	s.Cluster().Unregister("create")
	s.Cluster().Unregister("update")
	s.Cluster().Unregister("build")
	s.Cluster().Unregister("deploy")
	s.Cluster().Unsubscribe(event.MESSAGE_TOPIC_NAME)
	time.Sleep(1 * time.Second)
	s.AppManager.Shutdown()
}
