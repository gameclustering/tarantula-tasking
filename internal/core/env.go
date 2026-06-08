package core

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/rs/zerolog"
)

var (
	AppLog zerolog.Logger

	QueryFactoryRegistry = make(map[string]func() QueryFactory)
)

type Vault struct {
	Host  string
	Token string
}

type Env struct {
	Prefix         string `json:"Prefix"`
	Standalone     bool   `json:"Standalone"`
	GroupName      string `json:"GroupName"`
	NodeName       string `json:"NodeName"`
	NodeId         int64  `json:"NodeId"`
	PostOfficeHost string `json:"PostOfficeHost"`
	HttpBinding    string `json:"HttpBinding"`

	SqlEnabled       bool   `json:"SqlEnabled"`
	HomeDir          string `json:"HomeDir"`
	LogTruncated     bool   `json:"LogTruncated"`
	LogDir           string `json:"LogDir"`
	AuthLevel        int32  `json:"AuthLevel"`
	ClusterBootstrap string `json:"ClusterBootstrap"`
	IsClusterMember  bool   `json:"IsClusterMember"`

	Vlt Vault `json:"Vlt"`
}

func (f *Env) PresenceCtx() string {
	return f.Prefix + "/presence"
}

func (f *Env) Load(fn string) error {
	conf, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer conf.Close()
	data, _ := io.ReadAll(conf)
	json.Unmarshal(data, f)
	if f.HttpBinding == "" {
		f.HttpBinding = ":8080"
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	f.HomeDir = homeDir
	f.Prefix = "dev"
	c, exists := os.LookupEnv("POST_OFFICE_HOST")
	if exists {
		f.PostOfficeHost = c
	}

	c, exists = os.LookupEnv("ENV")
	if exists {
		f.Prefix = c
	}
	//use SEQ to keep nodeId unique in cluster scope
	//if no SEQ config, nodeId is fron the json config , also need to keep it unique in cluster scope
	c, exists = os.LookupEnv("SEQ")
	if exists {
		seq, err := strconv.Atoi(c)
		if err != nil {
			return err
		}
		f.NodeId = int64(f.NodeId + int64(seq))
		f.NodeName = fmt.Sprintf("%s.%d", f.GroupName, f.NodeId)

	}

	//f.Vlt = Vault{}
	c, exists = os.LookupEnv("CLUSTER_BOOTSTRAP")
	if exists {
		f.ClusterBootstrap = c
	}

	c, exists = os.LookupEnv("VAULT_HOST")
	if exists {
		f.Vlt.Host = c
	}
	c, exists = os.LookupEnv("VAULT_TOKEN")
	if exists {
		f.Vlt.Token = c
	}
	c, exists = os.LookupEnv("HTTP_BINDING")
	if exists {
		f.HttpBinding = c
	}
	return nil
}
