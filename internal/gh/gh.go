package gh

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
)

type RepoView struct {
	NameWithOwner string `json:"nameWithOwner"`
}

func DefaultHost() string {
	if host := strings.TrimSpace(os.Getenv("GH_HOST")); host != "" {
		return host
	}
	return "github.com"
}

func AuthToken(ctx context.Context, host string) (string, error) {
	args := []string{"auth", "token"}
	if host != "" {
		args = append(args, "--hostname", host)
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", errors.New("empty token from gh auth token")
	}
	return token, nil
}

func RepoViewCurrent(ctx context.Context) (RepoView, error) {
	cmd := exec.CommandContext(ctx, "gh", "repo", "view", "--json", "nameWithOwner")
	out, err := cmd.Output()
	if err != nil {
		return RepoView{}, err
	}
	var view RepoView
	if err := json.Unmarshal(out, &view); err != nil {
		return RepoView{}, err
	}
	if view.NameWithOwner == "" {
		return RepoView{}, errors.New("gh repo view returned empty nameWithOwner")
	}
	return view, nil
}
