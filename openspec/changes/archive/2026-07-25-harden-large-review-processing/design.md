## Context

The reviewer currently performs one list request for each GitHub or GitLab collection and sends every selected diff in one AI request. GitHub file payload fields are optional, while AI responses are untrusted JSON. The existing global run context already provides cancellation and a deadline, and the implementation should retain the current domain ports and standard-library-first style.

## Goals / Non-Goals

**Goals:**

- Retrieve all pages needed to review changes, detect existing comments, and delete prior bot comments.
- Prevent incomplete GitHub file payloads from causing panics.
- Keep individual AI requests near a configurable byte target while reviewing every available diff section.
- Reject malformed AI issues before invoking a VCS comment API.
- Preserve deterministic diff order, context cancellation, and current provider interfaces.

**Non-Goals:**

- Token counting or provider-specific context-window discovery.
- Parallel AI requests, retries, or rate-limit scheduling.
- Fetching repository blobs when a VCS does not provide a textual patch.
- Proving that an AI-provided line belongs to an added diff hunk.
- Adding providers, a daemon mode, or a persistent review database.

## Decisions

### Paginate inside each VCS adapter

Each adapter list method will loop using its SDK's response pagination metadata and mutate the SDK list options for the next page. GitHub and GitLab loops remain local because their option and response types differ; a shared pagination abstraction would add indirection without reusable behavior. Every request will continue using the caller's context. Maximum supported page sizes will reduce round trips without changing result ordering.

Alternative considered: rely on SDK defaults. This is the current behavior and returns incomplete results when more than one page exists.

### Skip GitHub file entries without reviewable patch data

The GitHub adapter will use generated getter methods instead of directly dereferencing optional fields. Entries with an empty filename or patch will be omitted because they cannot produce a grounded inline review. Other files remain reviewable, so one binary or oversized file does not fail the whole run.

Alternative considered: return an error for any missing patch. This would reduce coverage more severely by aborting otherwise reviewable changes. Fetching blobs is out of scope because it does not reconstruct a diff or its new-line positions.

### Batch rendered diff sections by a byte target

Runtime configuration will expose `MAX_DIFF_BYTES` as a positive integer with a default of `100000`. The application will render deterministically ordered diff sections, then append whole sections to the current batch until the next section would exceed the target. A section larger than the target will be sent alone and never truncated. Batches will be reviewed sequentially under the existing run context.

Byte measurement is intentionally used instead of approximate tokenization: it is deterministic, provider-neutral, and requires no dependency. The target is soft for a single oversized section so the reviewer never silently drops content.

Alternative considered: split patches inside a file. That requires hunk-aware parsing and can remove context needed for grounded findings. Provider-specific tokenizers were also rejected because they increase dependencies and cannot cover every supported model uniformly.

### Validate and normalize issues at the application boundary

Before publication, the reviewer will require a known file path, a positive line number, a non-empty trimmed message, and a case-insensitive severity in `error`, `warning`, or `info`. Severity will be normalized to lowercase and message whitespace trimmed before formatting. The existing single-known-file fallback for an omitted path remains. Invalid issues will be skipped with a structured warning so one malformed issue does not discard valid siblings.

Alternative considered: reject the whole model response. Per-issue rejection preserves useful findings and matches the current best-effort comment publication behavior.

## Risks / Trade-offs

- [Additional pages increase API calls and can consume more of `RUN_TIMEOUT`] → Request the largest supported page size and retain context cancellation on every call.
- [A later batch can fail after earlier comments were published] → Return the error and rely on the existing comment cleanup/idempotency behavior on rerun; cross-system rollback is not introduced.
- [Bytes only approximate model tokens] → Keep `MAX_DIFF_BYTES` configurable and document that a single oversized file can exceed the target.
- [Files without textual patches are not reviewed] → Continue reviewing all other files and cover the omission behavior explicitly in adapter tests.
- [Pagination can expose previously unseen comments and skip more files] → Treat this as corrected completeness rather than a compatibility break.

## Migration Plan

1. Add and document `MAX_DIFF_BYTES` with its default while preserving existing environment-only configuration.
2. Add pagination and incomplete-payload handling to both VCS adapters with HTTP-stub tests.
3. Add batching and issue validation to the application layer with focused tests.
4. Run the full race-enabled test suite and linter.

Rollback consists of reverting the change; no stored data or external migration is involved.

## Open Questions

None.
