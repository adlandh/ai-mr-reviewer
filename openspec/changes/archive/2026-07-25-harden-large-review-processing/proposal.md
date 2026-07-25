## Why

Large or unusual pull requests can currently be reviewed only partially, exceed an AI provider's practical input size, or trigger invalid comment requests. The reviewer should handle complete paginated data and untrusted API/model payloads predictably before more providers or features are added.

## What Changes

- Fetch every page of changed files and existing comments from GitHub and GitLab.
- Handle GitHub files whose optional filename or patch data is absent without panicking.
- Split review input into deterministic, size-targeted batches while preserving complete file diff sections and process each batch independently.
- Validate AI-produced issues before publishing them, skipping malformed file paths, line numbers, severities, or messages with a warning.
- Add focused adapter and application tests for pagination, missing patches, batching, and issue validation.

## Capabilities

### New Capabilities

- `complete-vcs-review-data`: Complete paginated retrieval of review changes and comments, with safe handling of incomplete file payloads.
- `bounded-review-batching`: Deterministic batching of large review input under a configurable byte target without silently dropping file diffs.
- `review-issue-validation`: Validation and safe rejection of malformed AI review issues before VCS publication.

### Modified Capabilities

None.

## Impact

- Affects the GitHub and GitLab adapters, reviewer orchestration, runtime configuration, and their tests.
- Large reviews may make multiple VCS list requests and multiple AI review requests within the existing run timeout.
- Adds one environment setting for the review batch byte target; no new dependencies or breaking interface changes are expected.
