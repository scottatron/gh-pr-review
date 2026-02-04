package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"gh-pr-review/internal/gh"
	"gh-pr-review/internal/github"
)

type reviewThread struct {
	ID            string              `json:"id"`
	IsResolved    bool                `json:"isResolved"`
	IsOutdated    bool                `json:"isOutdated"`
	Path          string              `json:"path"`
	Line          *int                `json:"line"`
	OriginalLine  *int                `json:"originalLine"`
	StartLine     *int                `json:"startLine"`
	OriginalStart *int                `json:"originalStartLine"`
	Comments      reviewThreadComment `json:"comments"`
}

type reviewThreadComment struct {
	Nodes []reviewComment `json:"nodes"`
}

type reviewComment struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	Author    struct {
		Login string `json:"login"`
	} `json:"author"`
}

type listResponse struct {
	Repository struct {
		PullRequest struct {
			ReviewThreads struct {
				PageInfo struct {
					HasNextPage bool    `json:"hasNextPage"`
					EndCursor   *string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []reviewThread `json:"nodes"`
			} `json:"reviewThreads"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	sub := os.Args[1]
	switch sub {
	case "list":
		if err := runList(os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "reply":
		if err := runReply(os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "resolve":
		if err := runResolve(os.Args[2:], true); err != nil {
			exitErr(err)
		}
	case "unresolve":
		if err := runResolve(os.Args[2:], false); err != nil {
			exitErr(err)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", sub)
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "gh-pr-review: manage GitHub PR review threads")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  gh-pr-review list --pr <number> [--repo owner/name] [--status all|resolved|unresolved|resolved-no-reply] [--host host] [--json]")
	fmt.Fprintln(os.Stdout, "  gh-pr-review reply --thread-id <id> --body <text> [--host host]")
	fmt.Fprintln(os.Stdout, "  gh-pr-review reply --thread-id <id> --body-file <path> [--host host]")
	fmt.Fprintln(os.Stdout, "  gh-pr-review resolve --thread-id <id> [--host host]")
	fmt.Fprintln(os.Stdout, "  gh-pr-review unresolve --thread-id <id> [--host host]")
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var repo string
	var pr int
	var status string
	var jsonOut bool
	var host string
	fs.StringVar(&repo, "repo", "", "owner/name (defaults to gh repo view)")
	fs.IntVar(&pr, "pr", 0, "PR number")
	fs.StringVar(&status, "status", "all", "all|resolved|unresolved|resolved-no-reply")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	fs.StringVar(&host, "host", gh.DefaultHost(), "GitHub host")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if pr <= 0 {
		return errors.New("--pr is required")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "all"
	}
	if status != "all" && status != "resolved" && status != "unresolved" && status != "resolved-no-reply" {
		return fmt.Errorf("invalid --status %q", status)
	}

	ctx := context.Background()
	owner, name, err := resolveRepo(ctx, repo)
	if err != nil {
		return err
	}
	token, err := gh.AuthToken(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to get gh auth token: %w", err)
	}
	client := github.NewClient(github.GraphQLEndpoint(host), token)

	threads, err := fetchAllThreads(ctx, client, owner, name, pr)
	if err != nil {
		return err
	}
	filtered := filterThreads(threads, status)
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(filtered)
	}
	printThreads(filtered)
	return nil
}

func runReply(args []string) error {
	fs := flag.NewFlagSet("reply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var threadID string
	var body string
	var bodyFile string
	var host string
	fs.StringVar(&threadID, "thread-id", "", "Review thread ID")
	fs.StringVar(&body, "body", "", "Reply body")
	fs.StringVar(&bodyFile, "body-file", "", "Read reply body from file")
	fs.StringVar(&host, "host", gh.DefaultHost(), "GitHub host")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if threadID == "" {
		return errors.New("--thread-id is required")
	}
	body, err := resolveBody(body, bodyFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("reply body is empty")
	}

	ctx := context.Background()
	token, err := gh.AuthToken(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to get gh auth token: %w", err)
	}
	client := github.NewClient(github.GraphQLEndpoint(host), token)
	return replyToThread(ctx, client, threadID, body)
}

func runResolve(args []string, resolve bool) error {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var threadID string
	var host string
	fs.StringVar(&threadID, "thread-id", "", "Review thread ID")
	fs.StringVar(&host, "host", gh.DefaultHost(), "GitHub host")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if threadID == "" {
		return errors.New("--thread-id is required")
	}

	ctx := context.Background()
	token, err := gh.AuthToken(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to get gh auth token: %w", err)
	}
	client := github.NewClient(github.GraphQLEndpoint(host), token)
	if resolve {
		return setThreadResolved(ctx, client, threadID, true)
	}
	return setThreadResolved(ctx, client, threadID, false)
}

func resolveRepo(ctx context.Context, repo string) (string, string, error) {
	if strings.TrimSpace(repo) == "" {
		view, err := gh.RepoViewCurrent(ctx)
		if err != nil {
			return "", "", fmt.Errorf("failed to resolve repo: %w", err)
		}
		repo = view.NameWithOwner
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo %q (expected owner/name)", repo)
	}
	return parts[0], parts[1], nil
}

func fetchAllThreads(ctx context.Context, client *github.Client, owner, name string, pr int) ([]reviewThread, error) {
	query := `query($owner:String!, $name:String!, $number:Int!, $after:String) {
  repository(owner:$owner, name:$name) {
    pullRequest(number:$number) {
      reviewThreads(first:100, after:$after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          isResolved
          isOutdated
          path
          line
          originalLine
          startLine
          originalStartLine
          comments(first:100) {
            nodes {
              id
              body
              createdAt
              author { login }
            }
          }
        }
      }
    }
  }
}`
	var all []reviewThread
	var after *string
	for {
		vars := map[string]interface{}{
			"owner":  owner,
			"name":   name,
			"number": pr,
			"after":  after,
		}
		var resp listResponse
		if err := client.Do(ctx, query, vars, &resp); err != nil {
			return nil, err
		}
		threads := resp.Repository.PullRequest.ReviewThreads.Nodes
		all = append(all, threads...)
		if !resp.Repository.PullRequest.ReviewThreads.PageInfo.HasNextPage {
			break
		}
		after = resp.Repository.PullRequest.ReviewThreads.PageInfo.EndCursor
		if after == nil || *after == "" {
			break
		}
	}
	return all, nil
}

func filterThreads(threads []reviewThread, status string) []reviewThread {
	if status == "all" {
		return threads
	}
	filtered := make([]reviewThread, 0, len(threads))
	for _, t := range threads {
		switch status {
		case "resolved":
			if t.IsResolved {
				filtered = append(filtered, t)
			}
		case "unresolved":
			if !t.IsResolved {
				filtered = append(filtered, t)
			}
		case "resolved-no-reply":
			if t.IsResolved && len(t.Comments.Nodes) <= 1 {
				filtered = append(filtered, t)
			}
		}
	}
	return filtered
}

func printThreads(threads []reviewThread) {
	if len(threads) == 0 {
		fmt.Fprintln(os.Stdout, "no review threads found")
		return
	}
	for _, t := range threads {
		status := "unresolved"
		if t.IsResolved {
			status = "resolved"
		}
		lineInfo := formatLineInfo(t)
		fmt.Fprintf(os.Stdout, "Thread %s (%s)%s\n", t.ID, status, lineInfo)
		for _, c := range t.Comments.Nodes {
			author := c.Author.Login
			if author == "" {
				author = "unknown"
			}
			fmt.Fprintf(os.Stdout, "  - %s at %s\n", author, c.CreatedAt)
			for _, line := range strings.Split(strings.TrimRight(c.Body, "\n"), "\n") {
				fmt.Fprintf(os.Stdout, "    %s\n", line)
			}
		}
	}
}

func formatLineInfo(t reviewThread) string {
	if t.Path == "" {
		return ""
	}
	parts := []string{t.Path}
	if t.StartLine != nil && t.Line != nil && *t.StartLine != *t.Line {
		parts = append(parts, fmt.Sprintf("%d-%d", *t.StartLine, *t.Line))
	} else if t.Line != nil {
		parts = append(parts, fmt.Sprintf("%d", *t.Line))
	} else if t.OriginalLine != nil {
		parts = append(parts, fmt.Sprintf("%d", *t.OriginalLine))
	}
	return fmt.Sprintf(" [%s]", strings.Join(parts, ":"))
}

func resolveBody(body, bodyFile string) (string, error) {
	if body != "" && bodyFile != "" {
		return "", errors.New("provide only one of --body or --body-file")
	}
	if bodyFile == "" {
		return body, nil
	}
	data, err := os.ReadFile(bodyFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func replyToThread(ctx context.Context, client *github.Client, threadID, body string) error {
	mutation := `mutation($threadId:ID!, $body:String!) {
  addPullRequestReviewThreadReply(input:{pullRequestReviewThreadId:$threadId, body:$body}) {
    comment { id }
  }
}`
	vars := map[string]interface{}{
		"threadId": threadID,
		"body":     body,
	}
	var resp struct {
		AddPullRequestReviewThreadReply struct {
			Comment struct {
				ID string `json:"id"`
			} `json:"comment"`
		} `json:"addPullRequestReviewThreadReply"`
	}
	if err := client.Do(ctx, mutation, vars, &resp); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "replied with comment id %s\n", resp.AddPullRequestReviewThreadReply.Comment.ID)
	return nil
}

func setThreadResolved(ctx context.Context, client *github.Client, threadID string, resolved bool) error {
	var mutation string
	var op string
	if resolved {
		op = "resolveReviewThread"
		mutation = `mutation($threadId:ID!) { resolveReviewThread(input:{threadId:$threadId}) { thread { id isResolved } } }`
	} else {
		op = "unresolveReviewThread"
		mutation = `mutation($threadId:ID!) { unresolveReviewThread(input:{threadId:$threadId}) { thread { id isResolved } } }`
	}
	vars := map[string]interface{}{
		"threadId": threadID,
	}
	var resp map[string]struct {
		Thread struct {
			ID         string `json:"id"`
			IsResolved bool   `json:"isResolved"`
		} `json:"thread"`
	}
	if err := client.Do(ctx, mutation, vars, &resp); err != nil {
		return err
	}
	result, ok := resp[op]
	if !ok {
		return errors.New("missing mutation response")
	}
	state := "unresolved"
	if result.Thread.IsResolved {
		state = "resolved"
	}
	fmt.Fprintf(os.Stdout, "thread %s is now %s\n", result.Thread.ID, state)
	return nil
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
