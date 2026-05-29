package util

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func TestPassword(t *testing.T) {
	h, err := HashPassword("password")
	if err != nil {
		t.Errorf("failed %s\n", err.Error())
	}
	er := ValidatePassword("password", h)
	if er != nil {
		t.Errorf("failed %s\n", er.Error())
	}
}

func TestPartition(t *testing.T) {
	p := Partition([]byte("hellp"), 5)
	if p > 5 {
		t.Errorf("falied partition %d\n", p)
	}
}

func TestKey(t *testing.T) {
	skey := KeyToBase64(Key(32))
	bkey, err := KeyFromBase64(skey)
	if err != nil {
		t.Errorf("bad format key %s\n", err.Error())
	}
	ckey := base64.StdEncoding.EncodeToString(bkey)
	if skey != ckey {
		t.Errorf("key not same %s %s \n", skey, ckey)
	}
}

func TestGitHubClient(t *testing.T) {
	vclient := VaultClient{Host: "https://gameclustering.com", Token: ""}
	err := vclient.Auth()
	if err != nil {
		t.Errorf("error %s", err.Error())
	}
	ak, err := vclient.Load("dev/presence", "git")
	if err != nil {
		t.Errorf("error %s", err.Error())
		return
	}
	gh := GitHubApi{Token: ak.Git.Token, Org: ak.Git.Org}
	repos, err := gh.ListRepos()

	if err != nil {
		t.Errorf("error %s", err.Error())
		return
	}
	fmt.Printf("repos %v\n", repos)

}

func TestGcpApi(t *testing.T) {
	vclient := VaultClient{Host: "https://gameclustering.com", Token: ""}
	err := vclient.Auth()
	if err != nil {
		t.Errorf("error %s", err.Error())
	}
	ak, err := vclient.Load("dev/presence", "gcp")
	if err != nil {
		t.Errorf("error %s", err.Error())
		return
	}
	cfg := ak.Gcp
	gcp := GcpApi{ServiceAccount: cfg.Iam, ProjectId: cfg.ProjectId, Zone: cfg.Zone}
	err = gcp.Auth()
	if err != nil {
		t.Errorf("error %s", err.Error())
		return
	}
	instanceName := fmt.Sprintf("%s-%d", cfg.Prefix, 1)
	err = gcp.Insert(instanceName, cfg.MachineType, cfg.ImageType)
	if err != nil {
		t.Errorf("error %s", err.Error())
		return
	}
	err = gcp.Delete(instanceName)
	if err != nil {
		t.Errorf("error %s", err.Error())
	}
}
