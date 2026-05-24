package main

import (
	"os"
	"time"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/event"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
)

type CloudService struct {
	bootstrap.AppManager
}

func (s *CloudService) Config() string {
	return "./cloud-conf.json"
	//return "/etc/tarantula/cloud-conf.json"
}

// https://dev.to/marrouchi/the-challenge-about-ssl-in-docker-containers-no-one-talks-about-32gh
func (s *CloudService) Start(f core.Env) error {
	s.AppManager.Start(f)
	s.createSchema()
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
	s.Cluster().Register("update", NewVMObejctUpdate(s))
	s.Cluster().Register("create", NewVMObejctCreate(s))
	s.Cluster().Register("check", NewRepositoryObejctCheck(s))

	s.Cluster().Register("updatex", &protocol.TccTransationListener{Reserve: func(e *protocol.Transaction) error {
		key, err := s.Cluster().AuthKey("gcp")
		if err != nil {
			return err
		}
		gcp := util.GcpApi{ServiceAccount: key.Gcp.Iam, ProjectId: "prismatic-grail-206205", Zone: "us-east1-c"}
		err = gcp.Auth()
		if err != nil {
			core.AppLog.Debug().Msgf("gcp auth error %s", err)
			return err
		}

		ins, err := gcp.Get("tarantula-build-02")
		if err != nil {
			core.AppLog.Debug().Msgf("gcp read error %s", err.Error())
			return err
		}

		ssh := util.SshClient{Host: ins.GetNetworkInterfaces()[0].AccessConfigs[0].GetNatIP(), User: "yinghu_lu", PrivateKey: key.Gcp.Ssh, KHFile: "../.ssh/known_hosts"}
		err = ssh.WithKey()
		if err != nil {
			core.AppLog.Debug().Msgf("gcp ssh error %s", err)
			return err
		}
		//var w bytes.Buffer

		gkey, err := s.Cluster().AuthKey("git")
		if err != nil {
			return err
		}
		err = os.WriteFile("id_ed25519", []byte(gkey.Git.Key), 0700)
		if err != nil {
			return err
		}
		f, err := os.Open("./id_ed25519")
		//err = ssh.Run("ssh-keyscan -t ed25519 github.com >> .ssh/known_hosts", &w)
		if err != nil {
			return err
		}
		err = ssh.Upload(f, "/home/yinghu_lu/.ssh/id_ed25519", "0700") //perm 0600
		if err != nil {
			core.AppLog.Debug().Msgf("scp ssh error %s", err)
			return err
		}
		//core.AppLog.Debug().Msgf("git known host added :%s", w.String())
		f.Close()
		gcp.Close()
		ssh.Close()
		return nil
	}, Confirm: func(e *protocol.Transaction) error {
		return nil
	}, Cancel: func(e *protocol.Transaction) error {
		return nil
	}})
	core.AppLog.Info().Msgf("Cloud service started %s", f.HttpBinding)
	return nil
}

func (s *CloudService) Shutdown() {
	s.Cluster().Unregister("check")
	s.Cluster().Unregister("create")
	s.Cluster().Unregister("update")
	s.Cluster().Unregister("updatex")
	time.Sleep(1 * time.Second)
	s.AppManager.Shutdown()
}
