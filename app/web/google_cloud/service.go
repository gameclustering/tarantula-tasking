package main

import (
	"net/http"
	"time"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/cloud"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/event"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
)

type GoogleCloudService struct {
	bootstrap.AppManager
}

func (s *GoogleCloudService) Config() string {
	return "/etc/tarantula/google_cloud-conf.json"
}

func (s *GoogleCloudService) Start(f core.Env) error {
	if err := s.AppManager.Start(f); err != nil {
		return err
	}
	store := cloud.NewStore(s.Sql)
	if err := store.CreateSchema(); err != nil {
		return err
	}

	s.Cluster().Subscribe(event.MESSAGE_TOPIC_NAME, &protocol.TopicEventListener{C: func() proto.Message {
		return &protocol.MessageEvent{}
	}, M: func(m proto.Message) {
		ro, ok := m.(*protocol.MessageEvent)
		if ok {
			core.AppLog.Debug().Msgf("MESSAGE event %s %s", ro.Message, ro.Source)
		}
	}})

	s.Cluster().Register("check_gcp",  cloud.NewCheck(&s.AppManager, store))
	s.Cluster().Register("create_gcp", cloud.NewCreate(&s.AppManager, store, "gcp", newGcpPlatform))
	s.Cluster().Register("update_gcp", cloud.NewUpdate(&s.AppManager, store, "gcp", newGcpPlatform))
	s.Cluster().Register("build_gcp",  cloud.NewBuild(&s.AppManager, store, "gcp", newGcpPlatform))
	s.Cluster().Register("deploy_gcp", cloud.NewDeploy(&s.AppManager, store, "gcp", newGcpPlatform))

	http.Handle("/cloud/meta/task/{taskId}", bootstrap.Logging(&cloudMetaGet{GoogleCloudService: s, store: store}))

	core.AppLog.Info().Msgf("Google Cloud service started %s", f.HttpBinding)
	return nil
}

func (s *GoogleCloudService) Shutdown() {
	s.Cluster().Unregister("check_gcp")
	s.Cluster().Unregister("create_gcp")
	s.Cluster().Unregister("update_gcp")
	s.Cluster().Unregister("build_gcp")
	s.Cluster().Unregister("deploy_gcp")
	s.Cluster().Unsubscribe(event.MESSAGE_TOPIC_NAME)
	time.Sleep(1 * time.Second)
	s.AppManager.Shutdown()
}
