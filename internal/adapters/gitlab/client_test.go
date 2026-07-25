package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/adlandh/ai-mr-reviewer/internal/domain"
	"github.com/adlandh/ai-mr-reviewer/internal/testutil/httpstub"
	gogitlab "gitlab.com/gitlab-org/api/client-go"
)

const testGitLabBaseURL = "https://gitlab.example.com/api/v4"
const testGitLabMRPath = "/api/v4/projects/123/merge_requests/5"
const testGitLabNotesPath = "/notes"
const testGitLabNotesPathPrefix = testGitLabMRPath + testGitLabNotesPath + "/"
const testCommentPrefix = "ai-mr-reviewer"
const errNewClient = "NewClient returned error: %v"
const errUnexpectedRequest = "unexpected request: %s %s"
const errCreateStubGitLabClient = "create stub gitlab client: %v"
const testNewGoPath = "new.go"

type discussionRequest struct {
	Body     string `json:"body"`
	Position struct {
		PositionType string `json:"position_type"`
		NewLine      int64  `json:"new_line"`
		HeadSHA      string `json:"head_sha"`
		BaseSHA      string `json:"base_sha"`
		StartSHA     string `json:"start_sha"`
		OldPath      string `json:"old_path"`
		NewPath      string `json:"new_path"`
	} `json:"position"`
}

func TestNewClientReturnsErrorForInvalidBaseURL(t *testing.T) {
	t.Parallel()

	_, err := NewClient(domain.GitLabConfig{URL: "://bad-url", Token: "token", ProjectID: "123"}, domain.RuntimeConfig{}, 5)
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestClientGetMergeRequestChangesReturnsDiffs(t *testing.T) {
	t.Parallel()

	client, err := NewClient(domain.GitLabConfig{URL: testGitLabBaseURL, Token: "token", ProjectID: "123"}, domain.RuntimeConfig{}, 5)
	if err != nil {
		t.Fatalf(errNewClient, err)
	}
	client.git, err = newStubGitLabClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != testGitLabMRPath+"/diffs" {
			t.Fatalf(errUnexpectedRequest, r.Method, r.URL.Path)
		}

		if r.URL.Query().Get("per_page") != "100" {
			t.Fatalf("unexpected per_page: %s", r.URL.Query().Get("per_page"))
		}

		switch r.URL.Query().Get("page") {
		case "":
			return gitLabJSONPage(fmt.Sprintf(`[
				{"old_path":"old.go","new_path":"%s","diff":"@@ -1 +1 @@"}
			]`, testNewGoPath), "2"), nil
		case "2":
			return gitLabJSONPage(`[
				{"old_path":"same.go","new_path":"same.go","diff":"@@ -2 +2 @@"}
			]`, ""), nil
		default:
			t.Fatalf("unexpected page: %s", r.URL.Query().Get("page"))
			return nil, nil
		}
	}))
	if err != nil {
		t.Fatalf(errCreateStubGitLabClient, err)
	}

	diffs, err := client.GetMergeRequestChanges(context.Background())
	if err != nil {
		t.Fatalf("GetMergeRequestChanges returned error: %v", err)
	}
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}
	if diffs[0].OldPath != "old.go" || diffs[0].NewPath != testNewGoPath || diffs[0].Content != "@@ -1 +1 @@" {
		t.Fatalf("unexpected first diff: %+v", diffs[0])
	}
}

func TestClientAddMergeRequestDiscussionIncludesPositionAndPrefix(t *testing.T) {
	t.Parallel()

	var got discussionRequest

	client, err := NewClient(domain.GitLabConfig{URL: testGitLabBaseURL, Token: "token", ProjectID: "123", CommitSHA: "head-sha", MergeRequestDiffBaseSHA: "base-sha"}, domain.RuntimeConfig{CommentPrefix: testCommentPrefix}, 5)
	if err != nil {
		t.Fatalf(errNewClient, err)
	}
	client.git, err = newStubGitLabClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != testGitLabMRPath+"/discussions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return httpstub.JSONResponse(http.StatusCreated, `{}`), nil
	}))
	if err != nil {
		t.Fatalf(errCreateStubGitLabClient, err)
	}

	err = client.AddMergeRequestDiscussion(context.Background(), "foo.go", 42, "please fix this")
	if err != nil {
		t.Fatalf("AddMergeRequestDiscussion returned error: %v", err)
	}

	assertDiscussionRequest(t, got)
}

func TestClientGetExistingCommentsReturnsOnlyNonSystemPositionedNotes(t *testing.T) {
	t.Parallel()

	client, err := NewClient(domain.GitLabConfig{URL: testGitLabBaseURL, Token: "token", ProjectID: "123"}, domain.RuntimeConfig{}, 5)
	if err != nil {
		t.Fatalf(errNewClient, err)
	}
	client.git, err = newStubGitLabClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != testGitLabMRPath+testGitLabNotesPath {
			t.Fatalf(errUnexpectedRequest, r.Method, r.URL.Path)
		}

		switch r.URL.Query().Get("page") {
		case "":
			return gitLabJSONPage(`[
				{"body":"first","system":false,"position":{"new_path":"foo.go","new_line":42}},
				{"body":"system","system":true,"position":{"new_path":"foo.go","new_line":42}},
				{"body":"missing-position","system":false}
			]`, "2"), nil
		case "2":
			return gitLabJSONPage(`[
				{"body":"second","system":false,"position":{"new_path":"foo.go","new_line":42}}
			]`, ""), nil
		default:
			t.Fatalf("unexpected page: %s", r.URL.Query().Get("page"))
			return nil, nil
		}
	}))
	if err != nil {
		t.Fatalf(errCreateStubGitLabClient, err)
	}

	got, err := client.GetExistingComments(context.Background())
	if err != nil {
		t.Fatalf("GetExistingComments returned error: %v", err)
	}
	if len(got) != 1 || len(got["foo.go:42"]) != 2 || got["foo.go:42"][0] != "first" || got["foo.go:42"][1] != "second" {
		t.Fatalf("unexpected comments map: %#v", got)
	}
}

func TestClientDeleteBotCommentsExceptResolvedDeletesOnlyUnresolvedBotNotes(t *testing.T) {
	t.Parallel()

	var deleted []string
	client, err := NewClient(domain.GitLabConfig{URL: testGitLabBaseURL, Token: "token", ProjectID: "123"}, domain.RuntimeConfig{CommentPrefix: testCommentPrefix}, 5)
	if err != nil {
		t.Fatalf(errNewClient, err)
	}
	client.git, err = gogitlab.NewClient(
		"token",
		gogitlab.WithBaseURL(testGitLabBaseURL),
		gogitlab.WithHTTPClient(&http.Client{
			Transport: httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == testGitLabMRPath+testGitLabNotesPath && r.URL.Query().Get("page") == "":
					return gitLabJSONPage(`[
							{"id":2,"body":"ai-mr-reviewer: resolved","resolved":true,"system":false},
							{"id":3,"body":"system note","resolved":false,"system":true},
							{"id":4,"body":"human note","resolved":false,"system":false}
						]`, "2"), nil
				case r.Method == http.MethodGet && r.URL.Path == testGitLabMRPath+testGitLabNotesPath && r.URL.Query().Get("page") == "2":
					return gitLabJSONPage(`[
							{"id":1,"body":"ai-mr-reviewer: first","resolved":false,"system":false}
						]`, ""), nil
				case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, testGitLabNotesPathPrefix):
					deleted = append(deleted, strings.TrimPrefix(r.URL.Path, testGitLabNotesPathPrefix))
					return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header)}, nil
				default:
					t.Fatalf(errUnexpectedRequest, r.Method, r.URL.Path)
					return nil, nil
				}
			}),
		}),
	)
	if err != nil {
		t.Fatalf(errCreateStubGitLabClient, err)
	}

	err = client.DeleteBotCommentsExceptResolved(context.Background())
	if err != nil {
		t.Fatalf("DeleteBotCommentsExceptResolved returned error: %v", err)
	}

	if len(deleted) != 1 || deleted[0] != "1" {
		t.Fatalf("unexpected deleted note ids: %v", deleted)
	}
}

func TestClientDeleteBotCommentsExceptResolvedReturnsDeleteError(t *testing.T) {
	t.Parallel()

	client, err := NewClient(domain.GitLabConfig{URL: testGitLabBaseURL, Token: "token", ProjectID: "123"}, domain.RuntimeConfig{CommentPrefix: testCommentPrefix}, 5)
	if err != nil {
		t.Fatalf(errNewClient, err)
	}
	client.git, err = newStubGitLabClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == testGitLabMRPath+testGitLabNotesPath:
			return httpstub.JSONResponse(http.StatusOK, `[
				{"id":1,"body":"ai-mr-reviewer: first","resolved":false,"system":false}
			]`), nil
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, testGitLabNotesPathPrefix):
			return nil, errors.New("delete failed")
		default:
			t.Fatalf(errUnexpectedRequest, r.Method, r.URL.Path)
			return nil, nil
		}
	}))
	if err != nil {
		t.Fatalf(errCreateStubGitLabClient, err)
	}

	if err := client.DeleteBotCommentsExceptResolved(context.Background()); err == nil {
		t.Fatal("expected delete error")
	}
}

func TestClientGetMergeRequestChangesReturnsLaterPageError(t *testing.T) {
	t.Parallel()

	client, err := NewClient(domain.GitLabConfig{URL: testGitLabBaseURL, Token: "token", ProjectID: "123"}, domain.RuntimeConfig{}, 5)
	if err != nil {
		t.Fatalf(errNewClient, err)
	}
	client.git, err = newStubGitLabClient(t, httpstub.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") == "2" {
			return nil, errors.New("later page failed")
		}

		return gitLabJSONPage(`[]`, "2"), nil
	}))
	if err != nil {
		t.Fatalf(errCreateStubGitLabClient, err)
	}

	if _, err := client.GetMergeRequestChanges(context.Background()); err == nil {
		t.Fatal("expected later page error")
	}
}

func newStubGitLabClient(t *testing.T, transport httpstub.RoundTripFunc) (*gogitlab.Client, error) {
	t.Helper()

	return gogitlab.NewClient(
		"token",
		gogitlab.WithBaseURL(testGitLabBaseURL),
		gogitlab.WithHTTPClient(&http.Client{
			Transport: transport,
		}),
	)
}

func assertDiscussionRequest(t *testing.T, got discussionRequest) {
	t.Helper()

	if got.Body != "ai-mr-reviewer: please fix this" {
		t.Fatalf("unexpected discussion body: %q", got.Body)
	}
	if got.Position.PositionType != "line" {
		t.Fatalf("unexpected position type: %q", got.Position.PositionType)
	}
	if got.Position.NewLine != 42 {
		t.Fatalf("unexpected new line: %d", got.Position.NewLine)
	}
	if got.Position.HeadSHA != "head-sha" || got.Position.BaseSHA != "base-sha" || got.Position.StartSHA != "head-sha" {
		t.Fatalf("unexpected SHAs: %+v", got.Position)
	}
	if got.Position.OldPath != "foo.go" || got.Position.NewPath != "foo.go" {
		t.Fatalf("unexpected paths: %+v", got.Position)
	}
}

func gitLabJSONPage(body, nextPage string) *http.Response {
	response := httpstub.JSONResponse(http.StatusOK, body)
	if nextPage != "" {
		response.Header.Set("X-Next-Page", nextPage)
	}

	return response
}
