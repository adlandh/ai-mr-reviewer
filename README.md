# AI MR Reviewer

`ai-mr-reviewer` is a Go CLI that reviews GitLab merge requests and GitHub pull requests with an AI model and publishes inline feedback back to the code review.

This is a hobby project. It is not production-ready and should be used with that expectation.

It supports:
- GitLab merge requests
- GitHub pull requests
- Ollama
- OpenAI-compatible endpoints
- Anthropic
- GitHub Models / Copilot-compatible inference

## What It Does

For each run, the tool:
1. loads repository and provider configuration from environment variables
2. fetches the current MR/PR diff from GitLab or GitHub
3. combines the diff into a single review prompt
4. sends that prompt to the configured AI provider
5. parses the model response into review issues
6. posts inline comments back to the MR/PR

It can also delete earlier unresolved bot comments before publishing a fresh review pass.

## Requirements

- Go `1.26+`
- access to either GitLab or GitHub APIs
- one configured AI provider:
  - Ollama
  - OpenAI
  - Anthropic
  - Copilot / GitHub Models

## Quick Start

### GitHub + OpenAI

```bash
export VCS_PROVIDER="github"
export GITHUB_TOKEN="your-github-token"
export GITHUB_OWNER="your-org-or-user"
export GITHUB_REPO="your-repo"
export GITHUB_PR_NUMBER="123"
export CI_COMMIT_SHA="abc123"

export AI_PROVIDER="openai"
export OPENAI_API_KEY="your-openai-key"

go run ./main.go
```

### GitLab + Ollama

```bash
export VCS_PROVIDER="gitlab"
export GITLAB_TOKEN="your-gitlab-token"
export CI_PROJECT_ID="123"
export CI_MERGE_REQUEST_IID="1"
export CI_COMMIT_SHA="abc123"
export CI_MERGE_REQUEST_DIFF_BASE_SHA="def456"

export AI_PROVIDER="ollama"
export OLLAMA_URL="http://localhost:11434"
export OLLAMA_MODEL="llama3.2"

go run ./main.go
```

## How Reviews Work

- The reviewer fetches every API page of changed files and existing comments.
- Diff sections are sorted by path and grouped into sequential AI requests near `MAX_DIFF_BYTES`.
- A single section larger than `MAX_DIFF_BYTES` is sent whole in its own request rather than truncated.
- Existing comments are checked so already-reviewed files can be skipped.
- The AI response is expected to contain JSON with an `issues` array.
- Valid issues become inline discussions; malformed issues are logged and skipped independently.
- `RUN_TIMEOUT` limits the full run, including API calls and model inference.

## Configuration

The application is configured entirely through environment variables.

### Core Settings

| Variable | Description | Default |
| --- | --- | --- |
| `VCS_PROVIDER` | Review source platform: `gitlab` or `github` | `gitlab` |
| `AI_PROVIDER` | AI backend: `ollama`, `openai`, `anthropic`, or `copilot` | `ollama` |
| `COMMENT_PREFIX` | Prefix added to every bot comment as `<prefix>:` | `ai-mr-reviewer` |
| `DELETE_BOT_COMMENTS` | Delete previous unresolved bot comments before reviewing | `true` |
| `RUN_TIMEOUT` | Maximum duration for a full review run | `10m` |
| `MAX_DIFF_BYTES` | Target maximum rendered diff bytes per AI request | `100000` |

### GitLab Settings

| Variable | Description | Default |
| --- | --- | --- |
| `GITLAB_TOKEN` | GitLab API token | - |
| `CI_PROJECT_ID` | GitLab project ID | - |
| `CI_MERGE_REQUEST_IID` | Merge request IID | - |
| `CI_COMMIT_SHA` | Commit SHA for the review target | - |
| `CI_MERGE_REQUEST_DIFF_BASE_SHA` | Base SHA for the MR diff | - |
| `CI_SERVER_URL` | GitLab server URL | `https://gitlab.com` |

### GitHub Settings

| Variable | Description | Default |
| --- | --- | --- |
| `GITHUB_TOKEN` | GitHub token for PR access and, for `copilot`, model access | - |
| `GITHUB_OWNER` | Repository owner | - |
| `GITHUB_REPO` | Repository name | - |
| `GITHUB_PR_NUMBER` | Pull request number | - |
| `CI_COMMIT_SHA` | Commit SHA for the review target | - |

### Ollama Settings

| Variable | Description | Default |
| --- | --- | --- |
| `OLLAMA_URL` | Ollama base URL | `http://localhost:11434` |
| `OLLAMA_API_KEY` | Optional API key for protected or hosted Ollama endpoints | - |
| `OLLAMA_MODEL` | Ollama model name | `llama3.2` |

### OpenAI Settings

| Variable | Description | Default |
| --- | --- | --- |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `OPENAI_BASE_URL` | OpenAI-compatible API base URL | `https://api.openai.com/v1` |
| `OPENAI_MODEL` | Model name | `GPT-5.2-Codex` |

### Anthropic Settings

| Variable | Description | Default |
| --- | --- | --- |
| `ANTHROPIC_AUTH_TOKEN` | Anthropic API token | - |
| `ANTHROPIC_BASE_URL` | Anthropic API base URL | `https://api.anthropic.com/v1/` |
| `ANTHROPIC_MODEL` | Model name | `claude-sonnet-4-20250514` |

### Copilot / GitHub Models Settings

| Variable | Description | Default |
| --- | --- | --- |
| `GITHUB_TOKEN` | GitHub token used as bearer auth for GitHub Models | - |
| `COPILOT_BASE_URL` | GitHub Models-compatible inference URL | `https://models.github.ai/inference` |
| `COPILOT_MODEL` | GitHub Models-compatible model name | `openai/gpt-4.1` |

## Local Usage Examples

### GitLab + Anthropic

```bash
export VCS_PROVIDER="gitlab"
export GITLAB_TOKEN="your-gitlab-token"
export CI_PROJECT_ID="123"
export CI_MERGE_REQUEST_IID="1"
export CI_COMMIT_SHA="abc123"
export CI_MERGE_REQUEST_DIFF_BASE_SHA="def456"

export AI_PROVIDER="anthropic"
export ANTHROPIC_AUTH_TOKEN="your-anthropic-token"

go run ./main.go
```

### GitHub + Copilot

```bash
export VCS_PROVIDER="github"
export GITHUB_TOKEN="your-github-token"
export GITHUB_OWNER="your-org-or-user"
export GITHUB_REPO="your-repo"
export GITHUB_PR_NUMBER="123"
export CI_COMMIT_SHA="abc123"

export AI_PROVIDER="copilot"
export COPILOT_BASE_URL="https://models.github.ai/inference" # optional
export COPILOT_MODEL="openai/gpt-4.1"                         # optional

go run ./main.go
```

## CI Integration

### GitLab CI

```yaml
stages:
  - review

ai-code-review:
  stage: review
  image: golang:1.26-alpine
  services:
    - name: ollama/ollama:latest
      alias: ollama
  variables:
    VCS_PROVIDER: "gitlab"
    CI_SERVER_URL: $CI_SERVER_URL
    GITLAB_TOKEN: $GITLAB_TOKEN
    CI_PROJECT_ID: $CI_PROJECT_ID
    CI_MERGE_REQUEST_IID: $CI_MERGE_REQUEST_IID
    CI_COMMIT_SHA: $CI_COMMIT_SHA
    CI_MERGE_REQUEST_DIFF_BASE_SHA: $CI_MERGE_REQUEST_DIFF_BASE_SHA
    AI_PROVIDER: "ollama"
    OLLAMA_URL: "http://ollama:11434"
    OLLAMA_MODEL: "llama3.2"
  before_script:
    - go install github.com/adlandh/ai-mr-reviewer@latest
  script:
    - ai-mr-reviewer
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_MERGE_REQUEST_IID
  allow_failure: true
```

### GitHub Actions

Base workflow:

```yaml
name: AI Code Review

on:
  pull_request:

jobs:
  review:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
      models: read
    steps:
      - uses: actions/checkout@v6
      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: "1.26"
      - name: Install and run reviewer
        env:
          VCS_PROVIDER: github
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GITHUB_OWNER: ${{ github.repository_owner }}
          GITHUB_REPO: ${{ github.event.repository.name }}
          GITHUB_PR_NUMBER: ${{ github.event.pull_request.number }}
          CI_COMMIT_SHA: ${{ github.sha }}
          DELETE_BOT_COMMENTS: true
        run: go install github.com/adlandh/ai-mr-reviewer@latest && ai-mr-reviewer
```

Provider-specific additions:

OpenAI:

```yaml
          AI_PROVIDER: openai
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          OPENAI_MODEL: GPT-5.2-Codex
```

Anthropic:

```yaml
          AI_PROVIDER: anthropic
          ANTHROPIC_AUTH_TOKEN: ${{ secrets.ANTHROPIC_AUTH_TOKEN }}
          ANTHROPIC_MODEL: claude-sonnet-4-20250514
```

Ollama:

```yaml
          AI_PROVIDER: ollama
          OLLAMA_URL: http://localhost:11434
          OLLAMA_API_KEY: ${{ secrets.OLLAMA_API_KEY }} # optional
          OLLAMA_MODEL: llama3.2
```

If you want GitHub Actions to start an Ollama model locally, add:

```yaml
      - name: Run model
        uses: ai-action/ollama-action@v2
        with:
          model: llama3.2
```

Copilot / GitHub Models:

```yaml
          AI_PROVIDER: copilot
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          COPILOT_BASE_URL: https://models.github.ai/inference
          COPILOT_MODEL: openai/gpt-4.1
```

`models: read` is only required when using `AI_PROVIDER=copilot`, but leaving it enabled is harmless if you switch providers later.

## Token Setup

### GitLab

Create a GitLab token with enough scope to read merge request data and write discussions.

Recommended process:
1. Go to GitLab -> Settings -> Access Tokens
2. Create a token with the scopes your instance requires for MR read/write operations
3. Store it as `GITLAB_TOKEN`

If your GitLab instance distinguishes discussion-writing permissions, make sure the token can post MR discussions.

### GitHub

For local runs with a personal token:
1. Go to GitHub -> Settings -> Developer settings -> Personal access tokens
2. Create a token with repository access appropriate for your repo visibility
3. If using `AI_PROVIDER=copilot`, ensure the token can call GitHub Models
4. Export it as `GITHUB_TOKEN`

For GitHub Actions:
- use `${{ secrets.GITHUB_TOKEN }}` for repository access
- add `models: read` when using `AI_PROVIDER=copilot`

## Development

### Build

```bash
go install .
```

### Run

```bash
go run ./main.go
```

### Test

```bash
go test ./...
```

Single package:

```bash
go test ./internal/application
```

Single test:

```bash
go test ./internal/application -run '^TestRunReviewsOnlyNewDiffs$'
```

Race-enabled suite:

```bash
go test -v -race ./...
```

### Lint

```bash
golangci-lint run ./...
```

### Generate mocks

```bash
go generate ./...
```

## Architecture

The project follows a ports-and-adapters style:

```text
main.go
internal/
  application/   # review workflow orchestration
  domain/        # entities, prompts, and port interfaces
  adapters/
    ai/          # Ollama, OpenAI, Anthropic, Copilot adapters
    config/      # env-based configuration
    github/      # GitHub PR adapter
    gitlab/      # GitLab MR adapter
  testutil/      # test helpers
```

Important files:
- `main.go` - Fx wiring and startup flow
- `internal/application/reviewer.go` - main review orchestration
- `internal/domain/use_cases.go` - domain port definitions
- `internal/adapters/config/config.go` - environment-backed config

## Notes and Limitations

- Byte-targeted batching reduces context pressure, but one oversized file can still exceed a model's context window because file diffs are never truncated.
- GitHub files without a textual patch, such as binary files, are skipped while other files remain reviewable.
- On GitHub, inline comments can fall back depending on review API line-placement rules.
- The tool expects the AI response to be parseable into the expected JSON issue format.
- Unsupported VCS providers or AI providers fail early at startup.

## License

MIT
