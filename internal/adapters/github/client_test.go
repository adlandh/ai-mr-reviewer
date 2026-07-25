package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/adlandh/ai-mr-reviewer/internal/testutil/httpstub"
	gogithub "github.com/google/go-github/v82/github"
)

const testGitHubBaseURL = "https://api.github.test/"
const testGitHubPRPath = "/repos/acme/repo/pulls/7"
const testGitHubIssuePath = "/repos/acme/repo/issues/7"
const errNewClient = "NewClient returned error: %v"
const errUnexpectedRequest = "unexpected request: %s %s"
const testNewGoPath = "new.go"

func newTestGitHubAPIClient(t *testing.T, transport httpstub.RoundTripFunc) *gogithub.Client {
	t.Helper()

	apiClient := gogithub.NewClient(&http.Client{Transport: transport})

	baseURL, err := url.Parse(testGitHubBaseURL)
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	apiClient.BaseURL = baseURL

	return apiClient
}

func TestNewClientParsesPullRequestNumber(t *testing.T) {
	t.Parallel()

	client, err := NewClient("token", "acme", "repo", "7", "abc123", "ai-mr-reviewer")
	if err != nil {
		t.Fatalf(errNewClient, err)
	}
	if client.owner != "acme" || client.repo != "repo" || client.prNumber != 7 {
		t.Fatalf("unexpected client fields: %+v", client)
	}
}

func TestNewClientReturnsErrorForInvalidPullRequestNumber(t *testing.T) {
	t.Parallel()

	_, err := NewClient("token", "acme", "repo", "not-a-number", "abc123", "ai-mr-reviewer")
	if err == nil {
		t.Fatal("expected error for invalid PR number")
	}
}

func TestClientGetMergeRequestChanges(t *testing.T) {
	t.Parallel()

	apiClient := newTestGitHubAPIClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != testGitHubPRPath+"/files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if r.URL.Query().Get("per_page") != "100" {
			t.Fatalf("unexpected per_page: %s", r.URL.Query().Get("per_page"))
		}

		switch r.URL.Query().Get("page") {
		case "":
			return githubJSONPage(fmt.Sprintf(`[
				{"filename":"%s","patch":"@@ -1 +1 @@","previous_filename":"old.go"},
				{"filename":"binary.png"},
				{"patch":"@@ -3 +3 @@"}
			]`, testNewGoPath), testGitHubBaseURL+"repos/acme/repo/pulls/7/files?page=2"), nil
		case "2":
			return githubJSONPage(`[{"filename":"same.go","patch":"@@ -2 +2 @@"}]`, ""), nil
		default:
			t.Fatalf("unexpected page: %s", r.URL.Query().Get("page"))
			return nil, nil
		}
	}))

	client := &Client{
		client:   apiClient,
		owner:    "acme",
		repo:     "repo",
		prNumber: 7,
	}

	diffs, err := client.GetMergeRequestChanges(context.Background())
	if err != nil {
		t.Fatalf("GetMergeRequestChanges returned error: %v", err)
	}
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}
	if diffs[0].NewPath != testNewGoPath || diffs[0].OldPath != "old.go" || diffs[0].Content != "@@ -1 +1 @@" {
		t.Fatalf("unexpected first diff: %+v", diffs[0])
	}
	if diffs[1].NewPath != "same.go" || diffs[1].OldPath != "" || diffs[1].Content != "@@ -2 +2 @@" {
		t.Fatalf("unexpected second diff: %+v", diffs[1])
	}
}

func TestClientGetMergeRequestChangesReturnsError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("list files failed")
	apiClient := newTestGitHubAPIClient(t, httpstub.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, wantErr
	}))
	client := &Client{client: apiClient, owner: "acme", repo: "repo", prNumber: 7}

	_, err := client.GetMergeRequestChanges(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestClientAddMergeRequestDiscussionFallsBackToIssueComment(t *testing.T) {
	t.Parallel()

	var issueCommentBody string
	apiClient := newTestGitHubAPIClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == testGitHubPRPath+"/comments":
			return httpstub.JSONResponse(http.StatusUnprocessableEntity, `{"message":"validation failed"}`), nil
		case r.Method == http.MethodPost && r.URL.Path == testGitHubIssuePath+"/comments":
			var payload struct {
				Body string `json:"body"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode issue comment payload: %v", err)
			}
			issueCommentBody = payload.Body
			return httpstub.JSONResponse(http.StatusCreated, `{"id":1}`), nil
		default:
			t.Fatalf(errUnexpectedRequest, r.Method, r.URL.Path)
			return nil, nil
		}
	}))

	client := &Client{
		client:        apiClient,
		commentPrefix: "ai-mr-reviewer",
		owner:         "acme",
		repo:          "repo",
		commitSHA:     "abc123",
		prNumber:      7,
	}

	err := client.AddMergeRequestDiscussion(context.Background(), "foo.go", 12, "please fix this")
	if err != nil {
		t.Fatalf("AddMergeRequestDiscussion returned error: %v", err)
	}

	want := "ai-mr-reviewer: **File: foo.go**\n\nplease fix this"
	if issueCommentBody != want {
		t.Fatalf("unexpected fallback issue comment body: %q", issueCommentBody)
	}
}

func TestClientGetExistingCommentsReturnsReviewCommentsWithPathAndLine(t *testing.T) {
	t.Parallel()

	apiClient := newTestGitHubAPIClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != testGitHubPRPath+"/comments" {
			t.Fatalf(errUnexpectedRequest, r.Method, r.URL.Path)
		}

		switch r.URL.Query().Get("page") {
		case "":
			return githubJSONPage(`[
				{"path":"foo.go","line":12,"body":"first"},
				{"path":"bar.go","body":"ignored-without-line"},
				{"line":5,"body":"ignored-without-path"}
			]`, testGitHubBaseURL+"repos/acme/repo/pulls/7/comments?page=2"), nil
		case "2":
			return githubJSONPage(`[{"path":"foo.go","line":12,"body":"second"}]`, ""), nil
		default:
			t.Fatalf("unexpected page: %s", r.URL.Query().Get("page"))
			return nil, nil
		}
	}))

	client := &Client{client: apiClient, owner: "acme", repo: "repo", prNumber: 7}

	got, err := client.GetExistingComments(context.Background())
	if err != nil {
		t.Fatalf("GetExistingComments returned error: %v", err)
	}

	want := []string{"first", "second"}
	if len(got) != 1 || len(got["foo.go:12"]) != len(want) {
		t.Fatalf("unexpected comments map: %#v", got)
	}
	for i, body := range want {
		if got["foo.go:12"][i] != body {
			t.Fatalf("unexpected comment bodies: %#v", got["foo.go:12"])
		}
	}
}

func TestClientGetExistingCommentsReturnsError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("list review comments failed")
	apiClient := newTestGitHubAPIClient(t, httpstub.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, wantErr
	}))
	client := &Client{client: apiClient, owner: "acme", repo: "repo", prNumber: 7}

	_, err := client.GetExistingComments(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestClientDeleteBotCommentsExceptResolvedDeletesBotReviewAndIssueComments(t *testing.T) {
	t.Parallel()

	var deletedPaths []string
	apiClient := newTestGitHubAPIClient(t, deleteCommentsTransport(t, &deletedPaths))

	client := &Client{
		client:        apiClient,
		commentPrefix: "ai-mr-reviewer",
		owner:         "acme",
		repo:          "repo",
		prNumber:      7,
	}

	if err := client.DeleteBotCommentsExceptResolved(context.Background()); err != nil {
		t.Fatalf("DeleteBotCommentsExceptResolved returned error: %v", err)
	}

	if len(deletedPaths) != 2 {
		t.Fatalf("expected 2 deletions, got %v", deletedPaths)
	}
	if deletedPaths[0] != "/repos/acme/repo/pulls/comments/11" || deletedPaths[1] != "/repos/acme/repo/issues/comments/21" {
		t.Fatalf("unexpected deletions: %v", deletedPaths)
	}
}

func deleteCommentsTransport(t *testing.T, deletedPaths *[]string) httpstub.RoundTripFunc {
	t.Helper()

	pages := map[string]struct {
		body    string
		nextURL string
	}{
		testGitHubPRPath + "/comments#": {
			body: `[
				{"id":12,"body":"human review comment"},
				{"body":"missing id"}
			]`,
			nextURL: testGitHubBaseURL + "repos/acme/repo/pulls/7/comments?page=2",
		},
		testGitHubPRPath + "/comments#2": {
			body: `[{"id":11,"body":"ai-mr-reviewer: review comment"}]`,
		},
		testGitHubIssuePath + "/comments#": {
			body: `[
				{"id":22,"body":"human issue comment"},
				{"id":23}
			]`,
			nextURL: testGitHubBaseURL + "repos/acme/repo/issues/7/comments?page=2",
		},
		testGitHubIssuePath + "/comments#2": {
			body: `[{"id":21,"body":"ai-mr-reviewer: issue comment"}]`,
		},
	}

	return func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodDelete {
			*deletedPaths = append(*deletedPaths, r.URL.Path)
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header)}, nil
		}
		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedRequest, r.Method, r.URL.Path)
		}

		page, ok := pages[r.URL.Path+"#"+r.URL.Query().Get("page")]
		if !ok {
			t.Fatalf(errUnexpectedRequest, r.Method, r.URL.Path)
		}

		return githubJSONPage(page.body, page.nextURL), nil
	}
}

func TestClientDeleteBotCommentsExceptResolvedReturnsLaterPageError(t *testing.T) {
	t.Parallel()

	apiClient := newTestGitHubAPIClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != testGitHubPRPath+"/comments" {
			t.Fatalf(errUnexpectedRequest, r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("page") == "2" {
			return nil, errors.New("later page failed")
		}

		return githubJSONPage(`[]`, testGitHubBaseURL+"repos/acme/repo/pulls/7/comments?page=2"), nil
	}))
	client := &Client{client: apiClient, commentPrefix: "ai-mr-reviewer", owner: "acme", repo: "repo", prNumber: 7}

	if err := client.DeleteBotCommentsExceptResolved(context.Background()); err == nil {
		t.Fatal("expected later page error")
	}
}

func TestClientDeleteBotCommentsExceptResolvedReturnsIssueCommentListError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("list issue comments failed")
	apiClient := newTestGitHubAPIClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == testGitHubPRPath+"/comments" {
			return githubJSONPage(`[]`, ""), nil
		}

		return nil, wantErr
	}))
	client := &Client{client: apiClient, commentPrefix: "ai-mr-reviewer", owner: "acme", repo: "repo", prNumber: 7}

	err := client.DeleteBotCommentsExceptResolved(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestClientAddMergeRequestDiscussionReturnsErrorWhenFallbackFails(t *testing.T) {
	t.Parallel()

	apiClient := newTestGitHubAPIClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == testGitHubPRPath+"/comments":
			return nil, errors.New("review endpoint down")
		case r.Method == http.MethodPost && r.URL.Path == testGitHubIssuePath+"/comments":
			return nil, errors.New("issue endpoint down")
		default:
			t.Fatalf(errUnexpectedRequest, r.Method, r.URL.Path)
			return nil, nil
		}
	}))

	client := &Client{
		client:        apiClient,
		commentPrefix: "ai-mr-reviewer",
		owner:         "acme",
		repo:          "repo",
		commitSHA:     "abc123",
		prNumber:      7,
	}

	if err := client.AddMergeRequestDiscussion(context.Background(), "foo.go", 12, "please fix this"); err == nil {
		t.Fatal("expected fallback failure error")
	}
}

func githubJSONPage(body, nextURL string) *http.Response {
	response := httpstub.JSONResponse(http.StatusOK, body)
	if nextURL != "" {
		response.Header.Set("Link", fmt.Sprintf("<%s>; rel=\"next\"", nextURL))
	}

	return response
}
