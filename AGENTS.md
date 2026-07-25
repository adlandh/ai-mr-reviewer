# Repository Guide

## Architecture

- This is a single-module Go 1.26 CLI. `main.go` is the Uber Fx composition root; `fx.Invoke(runReview)` performs one review during startup and then the process exits.
- `internal/application/reviewer.go` owns review orchestration. Keep entities, typed config, and VCS/AI ports in `internal/domain`; implementations belong in `internal/adapters/{ai,config,github,gitlab}`.
- Configuration is environment-only. Parse environment variables only in `internal/adapters/config`; pass `internal/domain` config structs elsewhere.
- `internal/domain/mocks/*.gen.go` is generated from ports in `internal/domain/use_cases.go`. Never hand-edit it; `go generate ./...` runs Mockery through the `go.mod` tool directive, so no separate Mockery install is needed.

## Commands

- Run/build: `go run ./main.go` / `go install .`
- All tests: `go test ./...`
- One package: `go test ./internal/application`
- One test: `go test ./internal/application -run '^TestRunReviewsOnlyNewDiffs$'`
- CI-equivalent tests: `go test -race -coverprofile=coverage.txt -covermode=atomic ./...`
- Local lint: `golangci-lint run ./...`
- Regenerate after changing a domain port: `go generate ./...` before linting or testing.

## Verification Gotchas

- `.lefthook.yml` runs pre-commit checks serially as `go generate ./...`, `golangci-lint run`, then `go fix ./...`; pre-push runs `go test -v -race ./...`. Recheck the diff after `go fix` because it runs after lint.
- Local `.golangci.yml` sets `run.tests: false`, so local lint excludes `*_test.go`. CI replaces that file with `adlandh/golangci-lint-config` before linting; inspect the downloaded config when CI and local lint disagree.
- HTTP adapter tests use `internal/testutil/httpstub`, not live services. Config tests use `t.Setenv`; do not make those tests or subtests parallel.
- Release CI runs `ko build ... .`. Changes to startup or dependency wiring must continue to build from the package root, not only as `go run ./main.go`.
