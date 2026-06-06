package util

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func (v *VultrApi) getJson(path string, cb Callback) error {
	tr := &http.Transport{
		DisableKeepAlives:  true,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", VULTR_API_HOST, path), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+v.ApiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return cb(resp)
}
