# gh-pr-review

CLI tool to manage GitHub PR review threads using the GitHub GraphQL API and your existing `gh` CLI auth token.

## Requirements

- Go 1.21+ (or compatible)
- `gh` CLI authenticated (`gh auth login`)

## Usage

List review threads (all/resolved/unresolved):

```bash
go run . list --pr 123 --repo owner/name
go run . list --pr 123 --status unresolved
```

JSON output:

```bash
go run . list --pr 123 --json
```

Reply to a thread:

```bash
go run . reply --thread-id THREAD_ID --body "Thanks!"
```

Resolve/unresolve a thread:

```bash
go run . resolve --thread-id THREAD_ID
go run . unresolve --thread-id THREAD_ID
```

## Notes

- The tool uses `gh auth token` for auth and calls the GitHub GraphQL API directly.
- For GitHub Enterprise, pass `--host` (uses `https://HOST/api/graphql`) and ensure `gh auth token --hostname HOST` works.
- Thread listing currently fetches up to 100 comments per thread and paginates threads in batches of 100.
