---
name: gh-pr-review
description: Systematic workflow for addressing PR review comments using gh-pr-review CLI. Use when the user wants to work through PR review comments and prefers the gh-pr-review tool over helper scripts.
compatibility: Requires gh CLI auth, gh-pr-review on PATH, and network access to GitHub API.
metadata:
  author: scottatron
  version: "0.2.0"
---

# PR Review Comment Workflow (gh-pr-review CLI)

GitHub repo: https://github.com/scottatron/gh-pr-review

A systematic approach to addressing PR review comments efficiently: fetch unresolved threads, work through them one by one with the user, implement and verify each fix before moving to the next issue, and keep review state clean as each thread is resolved.

## Requirements

- GitHub CLI (`gh`) installed and authenticated
- `gh-pr-review` CLI on `PATH`
- Repository write permissions (for resolving threads)

## Helper Commands (gh-pr-review CLI)

### Fetch review threads
Fetch review threads for a PR with resolution status.

```bash
# Unresolved threads only
 gh-pr-review list --pr [PR_NUMBER] --status unresolved

# All threads
 gh-pr-review list --pr [PR_NUMBER] --status all

# Resolved threads without replies
 gh-pr-review list --pr [PR_NUMBER] --status resolved-no-reply
```

Use `--repo owner/name` if not running within the repo. Use `--json` for structured output.

### Reply to a thread

```bash
 gh-pr-review reply --thread-id "THREAD_ID" --body "Reply message"
```

### Resolve or unresolve a thread

```bash
 gh-pr-review resolve --thread-id "THREAD_ID"
 gh-pr-review unresolve --thread-id "THREAD_ID"
```

## Core Workflow

### 1. Discovery

Fetch all unresolved review threads:

```bash
 gh-pr-review list --pr [PR_NUMBER] --status unresolved --json
```

If you need all threads, use `--status all`.

**Key data points:**
- `thread.id` - GraphQL thread ID (needed for replies/resolve/unresolve)
- `thread.isResolved` - Resolution status
- `thread.path` - File path
- `thread.line` - Line number
- `comments[].body` / `comments[].author.login` - comment context

Identify the next thread to handle based on priority (critical bugs first, then correctness, then quality/docs). Keep the remaining threads queued, but only actively discuss one issue at a time.

### 2. Analysis

For the current thread only:

1. Understand the issue and its impact
2. Identify 2-4 resolution approaches with trade-offs
3. Recommend best approach based on codebase patterns
4. Read relevant code context (affected files, related patterns, docs)

### 3. Decision Collection

Present exactly one issue at a time. Do not batch multiple review comments into one decision step, and do not ask the user to approve a whole set of issues up front.

**Format:**
```
Issue #N: [Brief description]
File: path/to/file.ts:42
Severity: Critical/Medium/Low

Options:
1. [Quick fix] - [Trade-offs]
2. [Thorough fix] - [Trade-offs]
3. [Alternative] - [Trade-offs]

Recommendation: Option X because [reasoning]
```

Collect the user's decision for the current thread, then move immediately into implementation for that thread before discussing the next one.

**Key Principle:** The loop is always:
Fetch unresolved threads -> pick one thread -> discuss one thread -> implement one thread -> reply/resolve one thread -> verify -> move to the next thread.

### 4. Implementation

After the user decides how to handle the current thread:

1. Make the smallest coherent code or documentation change needed for that thread
2. Check for closely related code that must change with it to keep behavior consistent
3. Run the most relevant verification for that specific change
4. Reply to and resolve the thread as soon as the fix is ready:

```bash
 gh-pr-review reply --thread-id "THREAD_ID" --body "Fixed in collaboration with Codex - [brief description]"
 gh-pr-review resolve --thread-id "THREAD_ID"
```

5. Ensure the work is not left in a pending review state before moving on:
- Prefer `gh-pr-review reply`, which should auto-submit the review if GitHub associates the reply with a pending review.
- If any other tool created a pending review while addressing the thread, submit or clean it up before continuing.
6. Refresh the unresolved thread list and continue with the next highest-priority thread

Keep changes focused to the current thread. Do not silently absorb additional review issues into the same user decision unless they are inseparable from the fix.

### 5. Commit

Create a comprehensive commit message once the intended set of threads for the round is complete:

```
fix: address [source] PR review comments

[One-sentence summary of scope]

**Critical Fixes:**
- [Security/bug fixes]

**Code Quality:**
- [Refactoring, best practices]

**Documentation:**
- [Examples, guides, comments]

**Changes:**
- path/to/file: [what changed and why]

All addressed review threads in this round are resolved.

Relates to #[PR_NUMBER]
```

Commit and push:
```bash
 git add [files]
 git commit -m "[message above]"
 git push
```

### 6. Verification

Verify the targeted threads are resolved and no new unresolved threads were introduced:

```bash
 gh-pr-review list --pr [PR_NUMBER] --status unresolved --json
```

If threads remain unresolved, continue the one-by-one loop. Do not collapse back into a batch triage step.

## Multi-Round Strategy

For PRs with many comments (>10), split into rounds:

- **Round 1:** Critical (security, bugs, breaking changes)
- **Round 2:** Code quality (refactoring, performance, best practices)
- **Round 3:** Polish (docs, examples, style)

Each round still processes threads sequentially: Fetch -> pick one -> analyze -> decide -> implement -> reply/resolve -> verify -> repeat

## Quality Checkpoints

Before committing:
- All user decisions implemented correctly
- Threads were handled one by one, not as a batched approval step
- No pending review was left behind after replying or resolving
- No unintended side effects
- Related code updated for consistency
- Documentation reflects changes
- Tests pass
- Commit message is comprehensive

## Common Patterns

**Security:** Always prioritize (Round 1), create issue if complex, document considerations

**Naming/Style:** Check existing patterns, apply consistently, update style guide if new pattern

**Dependencies:** Consider version compatibility, check breaking changes, update lock files

**Documentation:** Fix incorrect examples, update guides/READMEs, add comments for complex changes
