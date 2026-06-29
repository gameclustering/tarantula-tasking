package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	VULTR_API_HOST string = "https://api.vultr.com/v2"
)

type VultrInstance struct {
	Id          string   `json:"id"`
	Label       string   `json:"label"`
	Hostname    string   `json:"hostname"`
	Region      string   `json:"region"`
	Plan        string   `json:"plan"`
	MainIP      string   `json:"main_ip"`
	Status      string   `json:"status"`
	PowerStatus string   `json:"power_status"`
	Tags        []string `json:"tags"`
}

type vultrInstancesResponse struct {
	Instances []VultrInstance `json:"instances"`
}

type VultrSshKey struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	SshKey string `json:"ssh_key"`
}

type vultrSshKeysResponse struct {
	SshKeys []VultrSshKey `json:"ssh_keys"`
}

type vultrCreateInstanceRequest struct {
	Region   string   `json:"region"`
	Plan     string   `json:"plan"`
	OsId     int      `json:"os_id"`
	Label    string   `json:"label"`
	Hostname string   `json:"hostname"`
	SshKeys  []string `json:"sshkey_id,omitempty"`
}

type vultrInstanceResponse struct {
	Instance VultrInstance `json:"instance"`
}

type VultrApi struct {
	ApiKey string
}

func (v *VultrApi) ListInstances() ([]VultrInstance, error) {
	var out vultrInstancesResponse
	err := v.getJson("instances", func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("wrong response code %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(&out)
	})
	return out.Instances, err
}

// GetInstance fetches a single instance by its Vultr ID.
func (v *VultrApi) GetInstance(id string) (*VultrInstance, error) {
	var out vultrInstanceResponse
	err := v.getJson(fmt.Sprintf("instances/%s", id), func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("get instance HTTP %d: %s", resp.StatusCode, b)
		}
		return json.NewDecoder(resp.Body).Decode(&out)
	})
	return &out.Instance, err
}

// GetInstanceByLabel finds an instance by its label. Returns an error if not found.
func (v *VultrApi) GetInstanceByLabel(label string) (*VultrInstance, error) {
	instances, err := v.ListInstances()
	if err != nil {
		return nil, err
	}
	for _, ins := range instances {
		if ins.Label == label {
			return &ins, nil
		}
	}
	return nil, fmt.Errorf("instance %q not found", label)
}

// CreateInstance provisions a new Vultr VPS. osId is the Vultr OS image ID
// (e.g. 1743 for Debian 12). sshKeyIds are optional Vultr SSH key IDs to
// attach so root key-based login works immediately.
func (v *VultrApi) CreateInstance(label, region, plan string, osId int, sshKeyIds []string) (*VultrInstance, error) {
	body := vultrCreateInstanceRequest{
		Region:   region,
		Plan:     plan,
		OsId:     osId,
		Label:    label,
		Hostname: label,
		SshKeys:  sshKeyIds,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var out vultrInstanceResponse
	if err := v.doJson("POST", "instances", bytes.NewReader(data), func(resp *http.Response) error {
		if resp.StatusCode != http.StatusAccepted {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("create instance HTTP %d: %s", resp.StatusCode, b)
		}
		return json.NewDecoder(resp.Body).Decode(&out)
	}); err != nil {
		return nil, err
	}
	return &out.Instance, nil
}

// DeleteInstance destroys a Vultr VPS by its API ID.
func (v *VultrApi) DeleteInstance(id string) error {
	return v.doJson("DELETE", fmt.Sprintf("instances/%s", id), nil, func(resp *http.Response) error {
		if resp.StatusCode != http.StatusNoContent {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("delete instance HTTP %d: %s", resp.StatusCode, b)
		}
		return nil
	})
}

// FindSshKeyId returns the Vultr SSH key ID that corresponds to the given PEM
// private key. It derives the public key locally, fetches all registered keys
// from the Vultr account, and matches by public key fingerprint. Returns ""
// if no match is found (instance will be created without an attached key).
func (v *VultrApi) FindSshKeyId(pemPrivateKey string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(pemPrivateKey))
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	localFP := ssh.FingerprintSHA256(signer.PublicKey())

	var out vultrSshKeysResponse
	if err := v.getJson("ssh-keys", func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("list ssh-keys HTTP %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(&out)
	}); err != nil {
		return "", err
	}

	for _, k := range out.SshKeys {
		pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(k.SshKey)))
		if err != nil {
			continue
		}
		if ssh.FingerprintSHA256(pub) == localFP {
			return k.Id, nil
		}
	}
	return "", nil // no matching key registered; caller may proceed without one
}

// OsIdFromSettings parses settings["osId"] into an int; returns 0 on missing/invalid.
func OsIdFromSettings(settings map[string]string) int {
	s, ok := settings["osId"]
	if !ok {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

func (v *VultrApi) getJson(path string, cb Callback) error {
	return v.doJson("GET", path, nil, cb)
}

func (v *VultrApi) doJson(method, path string, body io.Reader, cb Callback) error {
	tr := &http.Transport{
		DisableKeepAlives:  true,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest(method, fmt.Sprintf("%s/%s", VULTR_API_HOST, path), body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+v.ApiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return cb(resp)
}
