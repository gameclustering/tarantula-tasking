package util

import (
	"context"

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

// PutSecretMap writes all fields in a single KVv2 Put, replacing any prior version.
// Use this for service credentials where multiple fields must be stored atomically.
func (v *VaultClient) PutSecretMap(mountPath, path string, data map[string]any) error {
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
	case "sql", "postgresql":
		return v.toSqlKey(kv), nil
	case "redis":
		return v.toRedisKey(kv), nil
	case "gcp":
		return v.toGcpKey(kv), nil
	case "aws":
		return v.toAwsKey(kv), nil
	case "az":
		return v.toAzKey(kv), nil
	case "git":
		return v.toGitKey(kv), nil
	case "docker":
		return v.toDockerKey(kv), nil
	default:
		return v.toVpsKey(kv), nil
	}
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
	cert, _ := kv.Data["cert"].(string)
	port, _ := kv.Data["port"].(string)
	db, _ := kv.Data["db"].(string)
	ak := protocol.AuthKey{Sql: &protocol.SqlAccess{}}
	ak.Sql.User = user
	ak.Sql.Password = pwd
	ak.Sql.Host = host
	ak.Sql.Cert = cert
	ak.Sql.Port = port
	ak.Sql.Db = db
	return &ak
}

func (a *VaultClient) toRedisKey(kv *vault.KVSecret) *protocol.AuthKey {
	host, _ := kv.Data["host"].(string)
	port, _ := kv.Data["port"].(string)
	password, _ := kv.Data["password"].(string)
	ak := protocol.AuthKey{Redis: &protocol.RedisAccess{}}
	ak.Redis.Host = host
	ak.Redis.Port = port
	ak.Redis.Password = password
	return &ak
}

func (a *VaultClient) toGcpKey(kv *vault.KVSecret) *protocol.AuthKey {
	iam, _ := kv.Data["iam"].(string)
	ssh, _ := kv.Data["ssh"].(string)
	projectId, _ := kv.Data["projectId"].(string)
	user, _ := kv.Data["user"].(string)
	ak := protocol.AuthKey{Gcp: &protocol.GcpAccess{}}
	ak.Gcp.Iam = iam
	ak.Gcp.Ssh = ssh
	ak.Gcp.ProjectId = projectId
	ak.Gcp.User = user
	return &ak
}

func (a *VaultClient) toAwsKey(kv *vault.KVSecret) *protocol.AuthKey {
	accessKeyId, _ := kv.Data["accessKeyId"].(string)
	secretAccessKey, _ := kv.Data["secretAccessKey"].(string)
	region, _ := kv.Data["region"].(string)
	prefix, _ := kv.Data["prefix"].(string)
	instanceType, _ := kv.Data["type"].(string)
	user, _ := kv.Data["user"].(string)
	ak := protocol.AuthKey{Aws: &protocol.AwsAccess{}}
	ak.Aws.AccessKeyId = accessKeyId
	ak.Aws.SecretAccessKey = secretAccessKey
	ak.Aws.Region = region
	ak.Aws.Prefix = prefix
	ak.Aws.Type = instanceType
	ak.Aws.User = user
	return &ak
}

func (a *VaultClient) toAzKey(kv *vault.KVSecret) *protocol.AuthKey {
	tenantId, _ := kv.Data["tenantId"].(string)
	clientId, _ := kv.Data["clientId"].(string)
	clientSecret, _ := kv.Data["clientSecret"].(string)
	subscriptionId, _ := kv.Data["subscriptionId"].(string)
	resourceGroup, _ := kv.Data["resourceGroup"].(string)
	region, _ := kv.Data["region"].(string)
	prefix, _ := kv.Data["prefix"].(string)
	instanceType, _ := kv.Data["type"].(string)
	user, _ := kv.Data["user"].(string)
	ak := protocol.AuthKey{Az: &protocol.AzAccess{}}
	ak.Az.TenantId = tenantId
	ak.Az.ClientId = clientId
	ak.Az.ClientSecret = clientSecret
	ak.Az.SubscriptionId = subscriptionId
	ak.Az.ResourceGroup = resourceGroup
	ak.Az.Region = region
	ak.Az.Prefix = prefix
	ak.Az.Type = instanceType
	ak.Az.User = user
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

func (a *VaultClient) toDockerKey(kv *vault.KVSecret) *protocol.AuthKey {
	server, _ := kv.Data["server"].(string)
	username, _ := kv.Data["username"].(string)
	password, _ := kv.Data["password"].(string)
	email, _ := kv.Data["email"].(string)
	token, _ := kv.Data["token"].(string)
	ak := protocol.AuthKey{Docker: &protocol.DockerAccess{}}
	ak.Docker.Server = server
	ak.Docker.Username = username
	ak.Docker.Password = password
	ak.Docker.Email = email
	ak.Docker.Token = token
	return &ak
}

func (a VaultClient) toVpsKey(kv *vault.KVSecret) *protocol.AuthKey {
	ssh, _ := kv.Data["ssh"].(string)
	user, _ := kv.Data["user"].(string)
	apiKey, _ := kv.Data["apiKey"].(string)
	password,_ :=kv.Data["password"].(string)
	ak := protocol.AuthKey{Vps: &protocol.VpsAccess{}}
	ak.Vps.Ssh = ssh
	ak.Vps.User = user
	ak.Vps.ApiKey = apiKey
	ak.Vps.Password = password //SUDO password in case running sudo permission locally
	return &ak
}
