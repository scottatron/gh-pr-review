# gh-pr-review

CLI tool to manage GitHub PR review threads using the GitHub GraphQL API and your existing `gh` CLI auth token.

## Requirements

- Go 1.21+ (or compatible)
- `gh` CLI authenticated (`gh auth login`)

## Build

```bash
go build -o gh-pr-review .
```

Ensure `gh-pr-review` is on your `PATH` (for example, move it into `~/bin` or another directory already in `PATH`).

## Usage

List review threads (all/resolved/unresolved/resolved-no-reply):

```bash
gh-pr-review list --pr 123 --repo owner/name
gh-pr-review list --pr 123 --status unresolved
gh-pr-review list --pr 123 --status resolved-no-reply
```

JSON output:

```bash
gh-pr-review list --pr 123 --json
```

Reply to a thread:

```bash
gh-pr-review reply --thread-id THREAD_ID --body "Thanks!"
```

Resolve/unresolve a thread:

```bash
gh-pr-review resolve --thread-id THREAD_ID
gh-pr-review unresolve --thread-id THREAD_ID
```

## Notes

- The tool uses `gh auth token` for auth and calls the GitHub GraphQL API directly.
- For GitHub Enterprise, pass `--host` (uses `https://HOST/api/graphql`) and ensure `gh auth token --hostname HOST` works.
- Thread listing currently fetches up to 100 comments per thread and paginates threads in batches of 100.
