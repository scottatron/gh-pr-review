package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gh-pr-review/internal/github"
)

func TestReplyToThreadSubmitsPendingReview(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		var req github.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode graphql request: %v", err)
		}
		queries = append(queries, req.Query)

		switch {
		case strings.Contains(req.Query, "addPullRequestReviewThreadReply"):
			io.WriteString(w, `{"data":{"addPullRequestReviewThreadReply":{"comment":{"id":"COMMENT123","pullRequestReview":{"id":"REVIEW123","state":"PENDING"}}}}}`)
		case strings.Contains(req.Query, "submitPullRequestReview"):
			io.WriteString(w, `{"data":{"submitPullRequestReview":{"pullRequestReview":{"id":"REVIEW123","state":"COMMENTED"}}}}`)
		default:
			t.Fatalf("unexpected graphql query: %s", req.Query)
		}
	}))
	defer server.Close()

	client := github.NewClient(server.URL, "token")
	output := captureStdout(t, func() {
		if err := replyToThread(context.Background(), client, "THREAD123", "Thanks"); err != nil {
			t.Fatalf("replyToThread: %v", err)
		}
	})

	if len(queries) != 2 {
		t.Fatalf("expected 2 graphql calls, got %d", len(queries))
	}
	if !strings.Contains(output, "replied with comment id COMMENT123") {
		t.Fatalf("expected reply output, got %q", output)
	}
	if !strings.Contains(output, "submitted review REVIEW123 with state commented") {
		t.Fatalf("expected submitted-review output, got %q", output)
	}
}

func TestReplyToThreadLeavesSubmittedReviewsAlone(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		var req github.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode graphql request: %v", err)
		}
		queries = append(queries, req.Query)

		if !strings.Contains(req.Query, "addPullRequestReviewThreadReply") {
			t.Fatalf("unexpected graphql query: %s", req.Query)
		}

		io.WriteString(w, `{"data":{"addPullRequestReviewThreadReply":{"comment":{"id":"COMMENT456","pullRequestReview":{"id":"REVIEW456","state":"COMMENTED"}}}}}`)
	}))
	defer server.Close()

	client := github.NewClient(server.URL, "token")
	output := captureStdout(t, func() {
		if err := replyToThread(context.Background(), client, "THREAD456", "Already submitted"); err != nil {
			t.Fatalf("replyToThread: %v", err)
		}
	})

	if len(queries) != 1 {
		t.Fatalf("expected 1 graphql call, got %d", len(queries))
	}
	if !strings.Contains(output, "replied with comment id COMMENT456") {
		t.Fatalf("expected reply output, got %q", output)
	}
	if strings.Contains(output, "submitted review") {
		t.Fatalf("did not expect submitted-review output, got %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(data)
}

func TestInstallEmbeddedSkillCopiesSkill(t *testing.T) {
	tempDir := t.TempDir()

	installedPath, err := installEmbeddedSkill(tempDir)
	if err != nil {
		t.Fatalf("installEmbeddedSkill: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(installedPath, "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	want, err := embeddedSkillFiles.ReadFile("skills/gh-pr-review/SKILL.md")
	if err != nil {
		t.Fatalf("read embedded skill: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("installed skill content mismatch")
	}
}

func TestDefaultSkillInstallDirUsesHomeDirectory(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	got, err := defaultSkillInstallDir()
	if err != nil {
		t.Fatalf("defaultSkillInstallDir: %v", err)
	}

	want := filepath.Join(homeDir, ".agent", "skills")
	if got != want {
		t.Fatalf("defaultSkillInstallDir = %q, want %q", got, want)
	}
}

func TestRunInstallSkillUsesDefaultDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	output := captureStdout(t, func() {
		if err := runInstallSkill(nil); err != nil {
			t.Fatalf("runInstallSkill: %v", err)
		}
	})

	installedPath := filepath.Join(homeDir, ".agent", "skills", embeddedSkillName, "SKILL.md")
	if _, err := os.Stat(installedPath); err != nil {
		t.Fatalf("expected installed skill at %s: %v", installedPath, err)
	}
	if !strings.Contains(output, "installed skill") {
		t.Fatalf("expected install output, got %q", output)
	}
}
