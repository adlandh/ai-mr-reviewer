package github

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/v82/github"

	"github.com/adlandh/ai-mr-reviewer/internal/domain"
)

type Client struct {
	client        *github.Client
	commentPrefix string
	owner         string
	repo          string
	commitSHA     string
	prNumber      int
}

const listPageSize = 100

func NewClient(token, owner, repo, prNumber, commitSHA, commentPrefix string) (*Client, error) {
	httpClient := &http.Client{}
	client := github.NewClient(httpClient)
	client = client.WithAuthToken(token)

	prNum, err := strconv.Atoi(prNumber)
	if err != nil {
		return nil, fmt.Errorf("parse PR number: %w", err)
	}

	return &Client{
		client:        client,
		owner:         owner,
		repo:          repo,
		prNumber:      prNum,
		commitSHA:     commitSHA,
		commentPrefix: commentPrefix,
	}, nil
}

func (c *Client) GetMergeRequestChanges(ctx context.Context) ([]domain.Diff, error) {
	options := &github.ListOptions{PerPage: listPageSize}

	var files []*github.CommitFile

	for {
		page, response, err := c.client.PullRequests.ListFiles(ctx, c.owner, c.repo, c.prNumber, options)
		if err != nil {
			return nil, fmt.Errorf("failed to get PR files: %w", err)
		}

		files = append(files, page...)

		if response == nil || response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	var diffs []domain.Diff

	for _, f := range files {
		filename := f.GetFilename()

		patch := f.GetPatch()
		if filename == "" || patch == "" {
			continue
		}

		diffs = append(diffs, domain.Diff{
			NewPath: filename,
			OldPath: f.GetPreviousFilename(),
			Content: patch,
		})
	}

	return diffs, nil
}

func (c *Client) GetExistingComments(ctx context.Context) (map[string][]string, error) {
	existing := make(map[string][]string)

	reviewComments, err := c.listReviewComments(ctx)
	if err != nil {
		return nil, err
	}

	for _, comment := range reviewComments {
		if comment.Path != nil && comment.Line != nil {
			key := fmt.Sprintf("%s:%d", comment.GetPath(), comment.GetLine())
			existing[key] = append(existing[key], comment.GetBody())
		}
	}

	return existing, nil
}

func (c *Client) AddMergeRequestDiscussion(ctx context.Context, file string, line int, note string) error {
	prComment := &github.PullRequestComment{
		Body:     &note,
		Path:     &file,
		Line:     new(line),
		CommitID: &c.commitSHA,
		Side:     new("RIGHT"),
	}

	_, _, err := c.client.PullRequests.CreateComment(ctx, c.owner, c.repo, c.prNumber, prComment)
	if err != nil {
		body := fmt.Sprintf("%s: **File: %s**\n\n%s", c.commentPrefix, file, note)
		issueComment := &github.IssueComment{
			Body: &body,
		}

		_, _, err = c.client.Issues.CreateComment(ctx, c.owner, c.repo, c.prNumber, issueComment)
		if err != nil {
			return fmt.Errorf("failed to add PR comment: %w", err)
		}
	}

	return nil
}

func (c *Client) DeleteBotCommentsExceptResolved(ctx context.Context) error {
	err := c.deleteBotReviewComments(ctx)
	if err != nil {
		return err
	}

	return c.deleteBotIssueComments(ctx)
}

func (c *Client) deleteBotReviewComments(ctx context.Context) error {
	reviewComments, err := c.listReviewComments(ctx)
	if err != nil {
		return err
	}

	for _, comment := range reviewComments {
		if comment.ID == nil || comment.Body == nil {
			continue
		}

		if !strings.HasPrefix(*comment.Body, c.commentPrefix+":") {
			continue
		}

		_, err = c.client.PullRequests.DeleteComment(ctx, c.owner, c.repo, *comment.ID)
		if err != nil {
			return fmt.Errorf("failed to delete PR review comment: %w", err)
		}
	}

	return nil
}

func (c *Client) deleteBotIssueComments(ctx context.Context) error {
	issueComments, err := c.listIssueComments(ctx)
	if err != nil {
		return err
	}

	for _, comment := range issueComments {
		if comment.ID == nil || comment.Body == nil {
			continue
		}

		if !strings.HasPrefix(*comment.Body, c.commentPrefix+":") {
			continue
		}

		_, err = c.client.Issues.DeleteComment(ctx, c.owner, c.repo, *comment.ID)
		if err != nil {
			return fmt.Errorf("failed to delete PR issue comment: %w", err)
		}
	}

	return nil
}

func (c *Client) listReviewComments(ctx context.Context) ([]*github.PullRequestComment, error) {
	options := &github.PullRequestListCommentsOptions{ListOptions: github.ListOptions{PerPage: listPageSize}}

	var comments []*github.PullRequestComment

	for {
		page, response, err := c.client.PullRequests.ListComments(ctx, c.owner, c.repo, c.prNumber, options)
		if err != nil {
			return nil, fmt.Errorf("failed to list PR review comments: %w", err)
		}

		comments = append(comments, page...)

		if response == nil || response.NextPage == 0 {
			return comments, nil
		}

		options.Page = response.NextPage
	}
}

func (c *Client) listIssueComments(ctx context.Context) ([]*github.IssueComment, error) {
	options := &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: listPageSize}}

	var comments []*github.IssueComment

	for {
		page, response, err := c.client.Issues.ListComments(ctx, c.owner, c.repo, c.prNumber, options)
		if err != nil {
			return nil, fmt.Errorf("failed to list PR issue comments: %w", err)
		}

		comments = append(comments, page...)

		if response == nil || response.NextPage == 0 {
			return comments, nil
		}

		options.Page = response.NextPage
	}
}

var _ domain.MRProviderPort = (*Client)(nil)
