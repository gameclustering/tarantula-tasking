package util

import (
	"os/exec"
	"strings"
)

type GitResponse struct {
	Successful bool   `json:"successful"`
	Message    string `json:"message"`
}

func fromErr(err error) GitResponse {
	return GitResponse{Successful: false, Message: err.Error()}
}
func fromOut(out []byte) GitResponse {
	return GitResponse{Successful: true, Message: strings.ReplaceAll(string(out), "\n", "")}
}

func GitCurBranch() GitResponse {
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return fromErr(err)
	}
	return fromOut(out)
}

func GitAdd(fn string) GitResponse {
	cmd := exec.Command("git", "add", fn)
	out, err := cmd.Output()
	if err != nil {
		return fromErr(err)
	}
	return fromOut(out)
}

func GitCommit(msg string) GitResponse {
	cmd := exec.Command("git", "commit", ".", "-m", msg)
	out, err := cmd.Output()
	if err != nil {
		return fromErr(err)
	}
	return fromOut(out)
}

func GitPush() GitResponse {
	cmd := exec.Command("git", "push")
	out, err := cmd.Output()
	if err != nil {
		return fromErr(err)
	}
	return fromOut(out)
}

func GitPull() GitResponse {
	cmd := exec.Command("git", "pull")
	out, err := cmd.Output()
	if err != nil {
		return fromErr(err)
	}
	return fromOut(out)
}

func GitStatus() GitResponse {
	cmd := exec.Command("git", "status")
	out, err := cmd.Output()
	if err != nil {
		return fromErr(err)
	}
	return fromOut(out)
}

func GitCheckout(branch string) GitResponse {
	cmd := exec.Command("git", "checkout", branch)
	out, err := cmd.Output()
	if err != nil {
		return fromErr(err)
	}
	return fromOut(out)
}

func GitMerge(branch string) GitResponse {
	cmd := exec.Command("git", "merge", branch)
	out, err := cmd.Output()
	if err != nil {
		return fromErr(err)
	}
	return fromOut(out)
}
func GitRemove(fn string) GitResponse {
	cmd := exec.Command("git", "rm", fn)
	out, err := cmd.Output()
	if err != nil {
		return fromErr(err)
	}
	return fromOut(out)
}
