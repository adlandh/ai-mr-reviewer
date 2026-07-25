## ADDED Requirements

### Requirement: Changed-file retrieval follows pagination
The system SHALL retrieve changed-file or diff pages from the selected VCS until the provider reports that no next page exists, preserving provider page and item order in the returned diffs.

#### Scenario: GitHub pull request has multiple file pages
- **WHEN** GitHub reports a next page while listing pull request files
- **THEN** the system SHALL request each page and return reviewable diffs from every page

#### Scenario: GitLab merge request has multiple diff pages
- **WHEN** GitLab reports a next page while listing merge request diffs
- **THEN** the system SHALL request each page and return diffs from every page

### Requirement: Comment retrieval and cleanup follow pagination
The system SHALL inspect every page of VCS comments used for existing-comment detection or bot-comment cleanup.

#### Scenario: Existing bot comment is on a later page
- **WHEN** a matching positioned bot comment exists after the first comment page
- **THEN** the system SHALL include that comment in existing-comment detection

#### Scenario: Deletable bot comment is on a later page
- **WHEN** an unresolved matching bot comment exists after the first comment page
- **THEN** the system SHALL evaluate and delete it according to the existing cleanup rules

### Requirement: Incomplete GitHub file payloads are safe
The system MUST NOT panic when GitHub omits optional file fields and SHALL omit file entries that lack either a filename or a textual patch while retaining reviewable sibling files.

#### Scenario: GitHub omits a patch
- **WHEN** a GitHub file entry has no textual patch
- **THEN** the system SHALL skip that entry and return other reviewable file diffs

#### Scenario: GitHub omits a filename
- **WHEN** a GitHub file entry has no filename
- **THEN** the system SHALL skip that entry and return other reviewable file diffs

### Requirement: Pagination errors remain observable
The system MUST return an error with operation context when any requested VCS page cannot be retrieved.

#### Scenario: A later page request fails
- **WHEN** a provider returns an error while retrieving a page after the first
- **THEN** the current list operation SHALL fail with contextual error information

