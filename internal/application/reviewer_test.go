package application

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adlandh/ai-mr-reviewer/internal/domain"
	"github.com/adlandh/ai-mr-reviewer/internal/domain/mocks"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type addedDiscussion struct {
	file string
	line int
	body string
}

type reviewerHarness struct {
	runtime domain.RuntimeConfig
	mr      *mocks.MRProviderPort
	ai      *mocks.AIProviderPort
	logger  *zap.Logger
}

const (
	warningComment           = "ai-mr-reviewer:**WARNING**: fix it"
	expectedOneDiscussionFmt = "expected 1 discussion, got %d"
)

func TestParseReviewResponse(t *testing.T) {
	issues, _, err := parseReviewResponse("some text {\"issues\":[{\"file\":\"a.go\",\"line\":3,\"severity\":\"warning\",\"message\":\"x\"}]} tail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 || issues[0].Line != 3 || issues[0].FilePath != "a.go" {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestParseReviewResponseUsesPathFallback(t *testing.T) {
	issues, _, err := parseReviewResponse(`{"issues":[{"path":"a.go","line":4,"severity":"warning","message":"x"}]}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 || issues[0].FilePath != "a.go" || issues[0].Line != 4 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestParseReviewResponseExtractsJSONFromCodeFence(t *testing.T) {
	issues, _, err := parseReviewResponse("```json\n{\"issues\":[{\"file\":\"a.go\",\"line\":3,\"severity\":\"warning\",\"message\":\"x\"}]}\n```")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 || issues[0].FilePath != "a.go" {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestDetectLanguage(t *testing.T) {
	if got := detectLanguage("a.go"); got != "Go" {
		t.Fatalf("unexpected language: %s", got)
	}
	if got := detectLanguage("a.unknown"); got != "Unknown" {
		t.Fatalf("unexpected language: %s", got)
	}
}

func TestRunReviewsOnlyNewDiffs(t *testing.T) {
	h := newReviewerHarness(t, false)
	added := make([]addedDiscussion, 0, 1)

	h.mr.EXPECT().GetExistingComments(mock.Anything).Return(map[string][]string{
		"already.go:1": {warningComment},
	}, nil)
	h.mr.EXPECT().GetMergeRequestChanges(mock.Anything).Return([]domain.Diff{
		{NewPath: "already.go", Content: "diff1"},
		{NewPath: "new.go", Content: "diff2"},
	}, nil)
	h.ai.EXPECT().ReviewCode(mock.Anything, mock.Anything).Return(issueResponse(`{"file":"new.go","line":10,"severity":"warning","message":"fix it"}`), nil)
	h.mr.EXPECT().AddMergeRequestDiscussion(mock.Anything, "new.go", 10, warningComment).
		Run(func(_ context.Context, file string, line int, body string) {
			added = append(added, addedDiscussion{file: file, line: line, body: body})
		}).
		Return(nil)

	r := NewReviewer(h.runtime, h.mr, h.ai, h.logger)

	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(added) != 1 {
		t.Fatalf(expectedOneDiscussionFmt, len(added))
	}
	if added[0].file != "new.go" || added[0].line != 10 {
		t.Fatalf("unexpected discussion: %+v", added[0])
	}
}

func TestRunReviewsNewDiffsNoFilter(t *testing.T) {
	h := newReviewerHarness(t, false)
	callCount := 0

	expectSingleDiffReview(h, nil, nil, `{"file":"new.go","line":10,"severity":"warning","message":"fix it"}`)
	h.mr.EXPECT().AddMergeRequestDiscussion(mock.Anything, "new.go", 10, warningComment).
		Run(func(context.Context, string, int, string) { callCount++ }).
		Return(nil)

	r := NewReviewer(h.runtime, h.mr, h.ai, h.logger)

	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf(expectedOneDiscussionFmt, callCount)
	}
}

func TestRunContinuesWhenExistingCommentsFail(t *testing.T) {
	h := newReviewerHarness(t, false)
	callCount := 0

	expectSingleDiffReview(h, nil, context.DeadlineExceeded, `{"file":"new.go","line":10,"severity":"warning","message":"fix it"}`)
	h.mr.EXPECT().AddMergeRequestDiscussion(mock.Anything, "new.go", 10, warningComment).
		Run(func(context.Context, string, int, string) { callCount++ }).
		Return(nil)

	r := NewReviewer(h.runtime, h.mr, h.ai, h.logger)

	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf(expectedOneDiscussionFmt, callCount)
	}
}

func TestRunDeletesBotCommentsWhenEnabled(t *testing.T) {
	h := newReviewerHarness(t, true)
	deleteCalls := 0

	h.mr.EXPECT().DeleteBotCommentsExceptResolved(mock.Anything).
		Run(func(context.Context) { deleteCalls++ }).
		Return(nil)
	expectSingleDiffReview(h, nil, nil, "")

	r := NewReviewer(h.runtime, h.mr, h.ai, h.logger)

	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalls != 1 {
		t.Fatalf("expected delete call, got %d", deleteCalls)
	}
}

func TestRunUsesOnlyKnownDiffPathWhenIssueFileIsEmpty(t *testing.T) {
	h := newReviewerHarness(t, false)
	added := make([]addedDiscussion, 0, 1)

	expectSingleDiffReview(h, nil, nil, `{"line":10,"severity":"warning","message":"fix it"}`)
	h.mr.EXPECT().AddMergeRequestDiscussion(mock.Anything, "new.go", 10, warningComment).
		Run(func(_ context.Context, file string, line int, body string) {
			added = append(added, addedDiscussion{file: file, line: line, body: body})
		}).
		Return(nil)

	r := NewReviewer(h.runtime, h.mr, h.ai, h.logger)

	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(added) != 1 || added[0].file != "new.go" {
		t.Fatalf("unexpected discussions: %+v", added)
	}
}

func TestRunSkipsUnknownFilesFromAIResponse(t *testing.T) {
	h := newReviewerHarness(t, false)

	expectSingleDiffReview(h, nil, nil, `{"file":"other.go","line":10,"severity":"warning","message":"fix it"}`)

	r := NewReviewer(h.runtime, h.mr, h.ai, h.logger)

	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

}

func TestRunValidatesIssuesIndependently(t *testing.T) {
	h := newReviewerHarness(t, false)
	core, logs := observer.New(zap.WarnLevel)
	h.logger = zap.New(core)
	added := make([]addedDiscussion, 0, 1)

	h.mr.EXPECT().GetExistingComments(mock.Anything).Return(map[string][]string{}, nil)
	h.mr.EXPECT().GetMergeRequestChanges(mock.Anything).Return([]domain.Diff{{NewPath: "new.go", Content: "diff"}}, nil)
	h.ai.EXPECT().ReviewCode(mock.Anything, mock.Anything).Return(`{"issues":[
		{"line":10,"severity":" WARNING ","message":" fix it "},
		{"file":"new.go","line":"ten","severity":"warning","message":"bad type"},
		{"file":"new.go","line":0,"severity":"warning","message":"bad line"},
		{"file":"new.go","line":11,"severity":"critical","message":"bad severity"},
		{"file":"new.go","line":12,"severity":"info","message":"   "},
		{"file":"other.go","line":13,"severity":"error","message":"bad file"}
	]}`, nil)
	h.mr.EXPECT().AddMergeRequestDiscussion(mock.Anything, "new.go", 10, warningComment).
		Run(func(_ context.Context, file string, line int, body string) {
			added = append(added, addedDiscussion{file: file, line: line, body: body})
		}).
		Return(nil)

	if err := NewReviewer(h.runtime, h.mr, h.ai, h.logger).Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(added) != 1 {
		t.Fatalf(expectedOneDiscussionFmt, len(added))
	}
	if logs.Len() != 5 {
		t.Fatalf("expected 5 warnings, got %d", logs.Len())
	}
	malformedWarning := false
	for _, entry := range logs.All() {
		if entry.Message != "skip invalid issue" {
			t.Fatalf("unexpected warning: %s", entry.Message)
		}
		if entry.ContextMap()["reason"] == "malformed JSON" {
			malformedWarning = true
		}
	}
	if !malformedWarning {
		t.Fatal("expected malformed JSON warning")
	}
}

func TestRunSkipsMissingPathForMultipleFiles(t *testing.T) {
	h := newReviewerHarness(t, false)
	core, logs := observer.New(zap.WarnLevel)
	h.logger = zap.New(core)

	h.mr.EXPECT().GetExistingComments(mock.Anything).Return(map[string][]string{}, nil)
	h.mr.EXPECT().GetMergeRequestChanges(mock.Anything).Return([]domain.Diff{
		{NewPath: "a.go", Content: "diff-a"},
		{NewPath: "b.go", Content: "diff-b"},
	}, nil)
	h.ai.EXPECT().ReviewCode(mock.Anything, mock.Anything).Return(`{"issues":[{"line":10,"severity":"warning","message":"fix it"}]}`, nil)

	if err := NewReviewer(h.runtime, h.mr, h.ai, h.logger).Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logs.Len() != 1 || logs.All()[0].ContextMap()["reason"] != "unknown file" {
		t.Fatalf("unexpected warnings: %+v", logs.All())
	}
}

func TestBuildReviewBatchesSortsPathsAndDetectsLanguages(t *testing.T) {
	batches := buildReviewBatches([]domain.Diff{
		{NewPath: "b.unknown", Content: "diff-b-1"},
		{NewPath: "a.go", Content: "diff-a"},
		{NewPath: "b.unknown", Content: "diff-b-2"},
	}, 100000)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}

	want := "File: a.go\nLanguage: Go\nDiff:\ndiff-a\n\n" +
		"File: b.unknown\nLanguage: Unknown\nDiff:\ndiff-b-1\n\n" +
		"File: b.unknown\nLanguage: Unknown\nDiff:\ndiff-b-2"
	if batches[0].content != want {
		t.Fatalf("unexpected combined diff:\n%s", batches[0].content)
	}
}

func TestBuildReviewBatchesSplitsWholeSections(t *testing.T) {
	diffs := []domain.Diff{
		{NewPath: "c.go", Content: "diff-c"},
		{NewPath: "a.go", Content: "diff-a"},
		{NewPath: "b.go", Content: "diff-b"},
	}
	wantFirst := renderDiffSection(diffs[1]) + "\n\n" + renderDiffSection(diffs[2])

	batches := buildReviewBatches(diffs, len(wantFirst))
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	if batches[0].content != wantFirst || batches[1].content != renderDiffSection(diffs[0]) {
		t.Fatalf("unexpected batches: %+v", batches)
	}
}

func TestBuildReviewBatchesKeepsOversizedSectionsWhole(t *testing.T) {
	diffs := []domain.Diff{
		{NewPath: "a.go", Content: "diff-a"},
		{NewPath: "b.go", Content: "diff-b"},
	}

	batches := buildReviewBatches(diffs, 1)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	for i, batch := range batches {
		if batch.content != renderDiffSection(diffs[i]) {
			t.Fatalf("batch %d was truncated: %q", i, batch.content)
		}
	}
}

func TestRunReviewsAllBatches(t *testing.T) {
	h := newReviewerHarness(t, false)
	h.runtime.MaxDiffBytes = 1
	h.mr.EXPECT().GetExistingComments(mock.Anything).Return(map[string][]string{}, nil)
	h.mr.EXPECT().GetMergeRequestChanges(mock.Anything).Return([]domain.Diff{
		{NewPath: "b.go", Content: "diff-b"},
		{NewPath: "a.go", Content: "diff-a"},
	}, nil)
	h.ai.EXPECT().ReviewCode(mock.Anything, renderDiffSection(domain.Diff{NewPath: "a.go", Content: "diff-a"})).Return(issueResponse(""), nil)
	h.ai.EXPECT().ReviewCode(mock.Anything, renderDiffSection(domain.Diff{NewPath: "b.go", Content: "diff-b"})).Return(issueResponse(""), nil)

	if err := NewReviewer(h.runtime, h.mr, h.ai, h.logger).Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunStopsWhenBatchReviewFails(t *testing.T) {
	h := newReviewerHarness(t, false)
	h.runtime.MaxDiffBytes = 1
	h.mr.EXPECT().GetExistingComments(mock.Anything).Return(map[string][]string{}, nil)
	h.mr.EXPECT().GetMergeRequestChanges(mock.Anything).Return([]domain.Diff{
		{NewPath: "a.go", Content: "diff-a"},
		{NewPath: "b.go", Content: "diff-b"},
		{NewPath: "c.go", Content: "diff-c"},
	}, nil)
	h.ai.EXPECT().ReviewCode(mock.Anything, renderDiffSection(domain.Diff{NewPath: "a.go", Content: "diff-a"})).Return(issueResponse(""), nil)
	h.ai.EXPECT().ReviewCode(mock.Anything, renderDiffSection(domain.Diff{NewPath: "b.go", Content: "diff-b"})).Return("", context.DeadlineExceeded)

	err := NewReviewer(h.runtime, h.mr, h.ai, h.logger).Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "review batch 2/3") {
		t.Fatalf("expected second batch error, got %v", err)
	}
}

func TestRunReturnsInvalidReviewResponseError(t *testing.T) {
	h := newReviewerHarness(t, false)
	h.mr.EXPECT().GetExistingComments(mock.Anything).Return(map[string][]string{}, nil)
	h.mr.EXPECT().GetMergeRequestChanges(mock.Anything).Return([]domain.Diff{{NewPath: "new.go", Content: "diff"}}, nil)
	h.ai.EXPECT().ReviewCode(mock.Anything, mock.Anything).Return(`{"issues": invalid}`, nil)

	err := NewReviewer(h.runtime, h.mr, h.ai, h.logger).Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "parse review response") {
		t.Fatalf("expected parse review response error, got %v", err)
	}
}

func TestRunWarnsWhenAddingDiscussionFails(t *testing.T) {
	h := newReviewerHarness(t, false)
	core, logs := observer.New(zap.WarnLevel)
	h.logger = zap.New(core)

	expectSingleDiffReview(h, nil, nil, `{"file":"new.go","line":10,"severity":"warning","message":"fix it"}`)
	h.mr.EXPECT().AddMergeRequestDiscussion(mock.Anything, "new.go", 10, warningComment).Return(context.DeadlineExceeded)

	if err := NewReviewer(h.runtime, h.mr, h.ai, h.logger).Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logs.Len() != 1 || logs.All()[0].Message != "failed to add comment" {
		t.Fatalf("unexpected warnings: %+v", logs.All())
	}
}

func TestRunCancelsInFlightReview(t *testing.T) {
	h := newReviewerHarness(t, false)

	h.mr.EXPECT().GetExistingComments(mock.Anything).Return(map[string][]string{}, nil)
	h.mr.EXPECT().GetMergeRequestChanges(mock.Anything).Return([]domain.Diff{{NewPath: "new.go", Content: "diff2"}}, nil)
	h.ai.EXPECT().ReviewCode(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, _ string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})

	r := NewReviewer(h.runtime, h.mr, h.ai, h.logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := r.Run(ctx)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("expected deadline exceeded error, got %v", err)
	}
}

func newReviewerHarness(t *testing.T, deleteBotComments bool) reviewerHarness {
	t.Helper()

	return reviewerHarness{
		runtime: domain.RuntimeConfig{
			CommentPrefix:     "ai-mr-reviewer",
			DeleteBotComments: deleteBotComments,
			RunTimeout:        10 * time.Minute,
		},
		mr:     mocks.NewMRProviderPort(t),
		ai:     mocks.NewAIProviderPort(t),
		logger: zap.NewNop(),
	}
}

func expectSingleDiffReview(h reviewerHarness, comments map[string][]string, commentsErr error, issue string) {
	if comments == nil {
		comments = map[string][]string{}
	}

	h.mr.EXPECT().GetExistingComments(mock.Anything).Return(comments, commentsErr)
	h.mr.EXPECT().GetMergeRequestChanges(mock.Anything).Return([]domain.Diff{{NewPath: "new.go", Content: "diff2"}}, nil)
	h.ai.EXPECT().ReviewCode(mock.Anything, mock.Anything).Return(issueResponse(issue), nil)
}

func issueResponse(issue string) string {
	if issue == "" {
		return `{"issues":[]}`
	}

	return `{"issues":[` + issue + `]}`
}
