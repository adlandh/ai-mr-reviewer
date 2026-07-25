## ADDED Requirements

### Requirement: Publishable issues have valid fields
Before invoking a VCS comment API, the system MUST require each AI-produced issue to reference a known file, use a positive line number, contain a non-empty message, and use the severity `error`, `warning`, or `info` case-insensitively.

#### Scenario: Issue fields are valid
- **WHEN** an issue references a known file, has a line greater than zero, a non-empty message, and a supported severity
- **THEN** the system SHALL normalize its severity and publish the issue

#### Scenario: Issue line is not positive
- **WHEN** an issue has a zero or negative line number
- **THEN** the system SHALL skip the issue without invoking a VCS comment API

#### Scenario: Issue severity is unsupported
- **WHEN** an issue severity is not `error`, `warning`, or `info` after case normalization
- **THEN** the system SHALL skip the issue without invoking a VCS comment API

#### Scenario: Issue message is blank
- **WHEN** an issue message is empty or contains only whitespace
- **THEN** the system SHALL skip the issue without invoking a VCS comment API

#### Scenario: Issue file is unknown
- **WHEN** an issue references a file absent from its review batch
- **THEN** the system SHALL skip the issue without invoking a VCS comment API

### Requirement: Single-file path fallback is preserved
The system SHALL use the only known batch file when an otherwise valid issue omits its file path and the batch contains exactly one file.

#### Scenario: File path is omitted for a single-file batch
- **WHEN** an issue omits its file path and its batch contains exactly one known file
- **THEN** the system SHALL associate the issue with that file before validation and publication

#### Scenario: File path is omitted for a multi-file batch
- **WHEN** an issue omits its file path and its batch contains multiple files
- **THEN** the system SHALL skip the issue without invoking a VCS comment API

### Requirement: Invalid issues do not suppress valid siblings
The system SHALL validate issues independently and SHALL emit a structured warning for each skipped issue.

#### Scenario: Response contains valid and invalid issues
- **WHEN** one AI response contains both publishable and malformed issues
- **THEN** the system SHALL publish the valid issues and warn for each malformed issue
