package util

import (
	"context"
	"fmt"

	"gameclustering.com/internal/protocol"
	vault "github.com/hashicorp/vault/api"
)

type VaultClient struct {
	Host   string
	Token  string
	client *vault.Client
}

func (v *VaultClient) Auth() error {
	cfg := vault.DefaultConfig()
	cfg.Address = v.Host
	client, err := vault.NewClient(cfg)
	if err != nil {
		return err
	}
	client.SetToken(v.Token)
	v.client = client
	return nil
}

func (v *VaultClient) GetSecret(mountPath string, path string) (*vault.KVSecret, error) {
	return v.client.KVv2(mountPath).Get(context.Background(), path)
}

func (v *VaultClient) CreateKvStore(mountPath, description string) error {
	input := vault.MountInput{
		Type: "kv", Options: map[string]string{"version": "2"}, Description: description,
	}
	return v.client.Sys().Mount(mountPath, &input)
}

func (v *VaultClient) PutSecret(mountPath, path, key, value string) error {
	data := map[string]any{key: value}
	_, err := v.client.KVv2(mountPath).Put(context.Background(), path, data)
	return err
}

func (v *VaultClient) Load(mountPath string, path string) (*protocol.AuthKey, error) {
	ak := protocol.AuthKey{}
	kv, err := v.GetSecret(mountPath, path)
	if err != nil {
		return &ak, err
	}
	switch path {
	case "auth":
		return v.toAuthKey(kv), nil
	case "sql":
		return v.toSqlKey(kv), nil
	case "gcp":
		return v.toGcpKey(kv), nil
	case "git":
		return v.toGitKey(kv), nil
	}
	return &ak, fmt.Errorf("key path not existed")
}

func (a *VaultClient) toAuthKey(kv *vault.KVSecret) *protocol.AuthKey {
	jwt, _ := kv.Data["jwt"].(string)
	cipher, _ := kv.Data["cipher"].(string)
	key, _ := kv.Data["key"].(string)
	cert, _ := kv.Data["cert"].(string)

	ak := protocol.AuthKey{}
	ak.Jwt = jwt
	ak.Cipher = cipher
	ak.Key = key
	ak.Cert = cert
	return &ak
}

func (a *VaultClient) toSqlKey(kv *vault.KVSecret) *protocol.AuthKey {
	user, _ := kv.Data["user"].(string)
	pwd, _ := kv.Data["password"].(string)
	host, _ := kv.Data["host"].(string)
	ak := protocol.AuthKey{Sql: &protocol.SqlAccess{}}
	ak.Sql.User = user
	ak.Sql.Password = pwd
	ak.Sql.Host = host
	return &ak
}

func (a *VaultClient) toGcpKey(kv *vault.KVSecret) *protocol.AuthKey {
	iam, _ := kv.Data["iam"].(string)
	ssh, _ := kv.Data["ssh"].(string)
	ak := protocol.AuthKey{Gcp: &protocol.GcpAccess{}}
	ak.Gcp.Iam = iam
	ak.Gcp.Ssh = ssh
	return &ak
}

func (a *VaultClient) toGitKey(kv *vault.KVSecret) *protocol.AuthKey {
	key, _ := kv.Data["key"].(string)
	user, _ := kv.Data["user"].(string)
	email, _ := kv.Data["email"].(string)
	token, _ := kv.Data["token"].(string)
	org, _ := kv.Data["org"].(string)
	ak := protocol.AuthKey{Git: &protocol.GitAccess{}}
	ak.Git.Key = key
	ak.Git.User = user
	ak.Git.Email = email
	ak.Git.Token = token
	ak.Git.Org = org
	return &ak
}
