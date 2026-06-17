package main

import (
	"net/http"
	"strconv"
	"time"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/cloud"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/event"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
)

type VultrCloudService struct {
	bootstrap.AppManager
}

func (s *VultrCloudService) Config() string {
	return "/etc/tarantula/cloud-conf.json"
}

func (s *VultrCloudService) Start(f core.Env) error {
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

	s.Cluster().Register("check_vultr",  cloud.NewCheck(&s.AppManager, store))
	s.Cluster().Register("create_vultr", cloud.NewCreate(&s.AppManager, store, "vps", newVultrPlatform))
	s.Cluster().Register("update_vultr", cloud.NewUpdate(&s.AppManager, store, "vps", newVultrPlatform))
	s.Cluster().Register("build_vultr",  cloud.NewBuild(&s.AppManager, store, "vps", newVultrPlatform))
	s.Cluster().Register("deploy_vultr", cloud.NewDeploy(&s.AppManager, store, "vps", newVultrPlatform))

	http.Handle("/cloud/meta/task/{taskId}", bootstrap.Logging(&vultrMetaGet{VultrCloudService: s, store: store}))

	core.AppLog.Info().Msgf("Vultr Cloud service started %s", f.HttpBinding)
	return nil
}

func (s *VultrCloudService) Shutdown() {
	s.Cluster().Unregister("check_vultr")
	s.Cluster().Unregister("create_vultr")
	s.Cluster().Unregister("update_vultr")
	s.Cluster().Unregister("build_vultr")
	s.Cluster().Unregister("deploy_vultr")
	s.Cluster().Unsubscribe(event.MESSAGE_TOPIC_NAME)
	time.Sleep(1 * time.Second)
	s.AppManager.Shutdown()
}

type vultrMetaGet struct {
	*VultrCloudService
	store *cloud.Store
}

func (s *vultrMetaGet) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}

func (s *vultrMetaGet) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	taskIdStr := r.PathValue("taskId")
	taskId, err := strconv.ParseUint(taskIdStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid taskId"}))
		return
	}
	results, err := s.store.QueryByTaskId(taskId)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	w.Write(util.ToJson(results))
}
