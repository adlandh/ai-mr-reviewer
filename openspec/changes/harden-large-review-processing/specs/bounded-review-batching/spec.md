## ADDED Requirements

### Requirement: Review batch target is configurable
The system SHALL expose `MAX_DIFF_BYTES` as a positive integer review-batch target and SHALL use `100000` bytes when the variable is unset.

#### Scenario: Batch target is not configured
- **WHEN** `MAX_DIFF_BYTES` is unset
- **THEN** runtime configuration SHALL set the review-batch target to `100000`

#### Scenario: Batch target is invalid
- **WHEN** `MAX_DIFF_BYTES` is zero, negative, or not an integer
- **THEN** application startup SHALL fail with a configuration error

### Requirement: Diff sections are batched deterministically
The system SHALL order diff sections deterministically and group whole sections into sequential batches whose combined rendered size does not exceed the configured target unless one section alone exceeds it.

#### Scenario: Next section would exceed the target
- **WHEN** adding the next complete diff section would make the current batch exceed `MAX_DIFF_BYTES`
- **THEN** the system SHALL finish the current batch and place that section in the next batch

#### Scenario: Multiple sections fit in one batch
- **WHEN** complete consecutive diff sections fit within `MAX_DIFF_BYTES`
- **THEN** the system SHALL include them in one batch in deterministic path order

### Requirement: Available diff content is not silently discarded
The system MUST include every reviewable diff section in exactly one AI review batch and MUST NOT truncate a section to meet the byte target.

#### Scenario: One section exceeds the target
- **WHEN** a single rendered diff section is larger than `MAX_DIFF_BYTES`
- **THEN** the system SHALL send that complete section as its own batch

### Requirement: Review batches use existing run control
The system SHALL review batches sequentially with the existing run context and SHALL validate issues against only the files present in the corresponding batch.

#### Scenario: Multiple batches complete successfully
- **WHEN** a review produces multiple batches and every AI request succeeds
- **THEN** the system SHALL process and publish valid issues from every batch

#### Scenario: A batch review fails
- **WHEN** an AI request for a batch returns an error or the run context is canceled
- **THEN** the review run SHALL stop and return a contextual error

