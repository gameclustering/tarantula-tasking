package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	GIT_HUB_API_HOST    string = "https://api.github.com"
	GIT_HUB_API_ACCEPT  string = "application/vnd.github+json"
	GIT_HUB_API_VERSION string = "2026-03-10"
)

type Callback func(resp *http.Response) error

type Repo struct {
	Id     int64  `json:"id"`
	Name   string `json:"name"`
	GitUrl string `json:"git_url"`
}

type GitHubApi struct {
	Token string
	Org   string
}

func (h *GitHubApi) ListRepos() ([]Repo, error) {
	repos := make([]Repo, 0)
	err := h.getJson(fmt.Sprintf("orgs/%s/repos", h.Org), func(resp *http.Response) error {
		if resp.StatusCode != 200 {
			return fmt.Errorf("wrong response code %d %s", resp.StatusCode, h.Org)
		}
		return json.NewDecoder(resp.Body).Decode(&repos)
	})
	return repos, err
}

func (h *GitHubApi) getJson(path string, cb Callback) error {

	tr := &http.Transport{
		DisableKeepAlives:  true,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}
	url := fmt.Sprintf("%s/%s", GIT_HUB_API_HOST, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", GIT_HUB_API_ACCEPT)
	req.Header.Set("X-GitHub-Api-Version", GIT_HUB_API_VERSION)
	req.Header.Set("User-Agent", h.Org)
	req.Header.Set("Authorization", "Bearer "+h.Token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return cb(resp)
}

func (h *GitHubApi) PostJson(path string, payload any, cb Callback) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tr := &http.Transport{
		DisableKeepAlives:  true,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s", GIT_HUB_API_HOST, path), bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", GIT_HUB_API_ACCEPT)
	req.Header.Set("X-GitHub-Api-Version", GIT_HUB_API_VERSION)
	req.Header.Set("User-Agent", h.Org)
	req.Header.Set("Authorization", "Bearer "+h.Token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return cb(resp)
}
