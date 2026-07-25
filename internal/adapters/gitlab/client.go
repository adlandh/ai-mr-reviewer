package gitlab

import (
	"context"
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/adlandh/ai-mr-reviewer/internal/domain"
)

type Client struct {
	git     *gitlab.Client
	gitlab  domain.GitLabConfig
	runtime domain.RuntimeConfig
	iid     int
}

const listPageSize = 100

func NewClient(config domain.GitLabConfig, runtime domain.RuntimeConfig, iid int) (*Client, error) {
	git, err := gitlab.NewClient(config.Token, gitlab.WithBaseURL(config.URL))
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	return &Client{
		git:     git,
		iid:     iid,
		runtime: runtime,
		gitlab:  config,
	}, nil
}

func (c *Client) GetMergeRequestChanges(ctx context.Context) ([]domain.Diff, error) {
	options := &gitlab.ListMergeRequestDiffsOptions{ListOptions: gitlab.ListOptions{PerPage: listPageSize}}

	var changes []*gitlab.MergeRequestDiff

	for {
		page, response, err := c.git.MergeRequests.ListMergeRequestDiffs(
			c.gitlab.ProjectID,
			int64(c.iid),
			options,
			gitlab.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get MR changes: %w", err)
		}

		changes = append(changes, page...)

		if response == nil || response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	var diffs []domain.Diff
	for _, d := range changes {
		diffs = append(diffs, domain.Diff{
			NewPath: d.NewPath,
			OldPath: d.OldPath,
			Content: d.Diff,
		})
	}

	return diffs, nil
}

func (c *Client) GetExistingComments(ctx context.Context) (map[string][]string, error) {
	notes, err := c.listMergeRequestNotes(ctx)
	if err != nil {
		return nil, err
	}

	existing := make(map[string][]string)

	for _, note := range notes {
		if note.System {
			continue
		}

		if note.Position == nil {
			continue
		}

		key := fmt.Sprintf("%s:%d", note.Position.NewPath, note.Position.NewLine)
		existing[key] = append(existing[key], note.Body)
	}

	return existing, nil
}

func (c *Client) AddMergeRequestDiscussion(ctx context.Context, file string, line int, note string) error {
	commitSHA := c.gitlab.CommitSHA
	baseSHA := c.gitlab.MergeRequestDiffBaseSHA
	positionType := "line"

	line64 := int64(line)

	noteWithPrefix := c.runtime.CommentPrefix + ": " + note

	_, _, err := c.git.Discussions.CreateMergeRequestDiscussion(
		c.gitlab.ProjectID,
		int64(c.iid),
		&gitlab.CreateMergeRequestDiscussionOptions{
			Body: &noteWithPrefix,
			Position: &gitlab.PositionOptions{
				PositionType: &positionType,
				NewLine:      &line64,
				HeadSHA:      &commitSHA,
				BaseSHA:      &baseSHA,
				StartSHA:     &commitSHA,
				OldPath:      &file,
				NewPath:      &file,
			},
		},
		gitlab.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to add discussion: %w", err)
	}

	return nil
}

func (c *Client) DeleteBotCommentsExceptResolved(ctx context.Context) error {
	notes, err := c.listMergeRequestNotes(ctx)
	if err != nil {
		return err
	}

	for _, note := range notes {
		if note.System || note.Resolved {
			continue
		}

		if !strings.HasPrefix(note.Body, c.runtime.CommentPrefix+":") {
			continue
		}

		_, err := c.git.Notes.DeleteMergeRequestNote(c.gitlab.ProjectID, int64(c.iid), note.ID, gitlab.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("failed to delete note: %w", err)
		}
	}

	return nil
}

func (c *Client) listMergeRequestNotes(ctx context.Context) ([]*gitlab.Note, error) {
	options := &gitlab.ListMergeRequestNotesOptions{ListOptions: gitlab.ListOptions{PerPage: listPageSize}}

	var notes []*gitlab.Note

	for {
		page, response, err := c.git.Notes.ListMergeRequestNotes(
			c.gitlab.ProjectID,
			int64(c.iid),
			options,
			gitlab.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list notes: %w", err)
		}

		notes = append(notes, page...)

		if response == nil || response.NextPage == 0 {
			return notes, nil
		}

		options.Page = response.NextPage
	}
}

var _ domain.MRProviderPort = (*Client)(nil)
