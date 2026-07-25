## 1. Runtime Configuration and Batching

- [x] 1.1 Add the positive `MAX_DIFF_BYTES` setting with its `100000` default to environment parsing and typed runtime configuration, including default, override, and invalid-value tests.
- [x] 1.2 Refactor deterministic diff rendering into whole sections and group them into byte-targeted batches without truncating oversized sections.
- [x] 1.3 Process review batches sequentially under the existing context and add tests for fitting sections, split batches, oversized sections, error propagation, and complete coverage.

## 2. Complete VCS Retrieval

- [x] 2.1 Paginate GitHub changed-file and existing-comment retrieval using SDK pagination metadata while preserving result order.
- [x] 2.2 Paginate both GitHub bot-comment cleanup paths and add HTTP-stub coverage for comments found on later pages and later-page failures.
- [x] 2.3 Replace direct GitHub optional-field dereferences with safe getters, skip files without a filename or textual patch, and test valid siblings remain reviewable.
- [x] 2.4 Paginate GitLab diff retrieval, existing-note retrieval, and bot-note cleanup, with HTTP-stub coverage for later pages and later-page failures.

## 3. Review Issue Validation

- [x] 3.1 Validate and normalize file path, positive line, supported severity, and non-empty message before comment publication while preserving the single-file path fallback.
- [x] 3.2 Add application tests proving malformed issues are warned and skipped independently while valid sibling issues are published.

## 4. Documentation and Verification

- [x] 4.1 Document `MAX_DIFF_BYTES`, paginated retrieval, oversized-section behavior, and multi-request review execution in the README.
- [x] 4.2 Run `go test -race -count=1 ./...`, `go vet ./...`, `golangci-lint run ./...`, and `git diff --check`.
