package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/adlandh/ai-mr-reviewer/internal/domain"
	"github.com/adlandh/ai-mr-reviewer/internal/domain/mocks"
	"github.com/stretchr/testify/mock"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func TestMainFunctionExists(t *testing.T) {
	t.Parallel()

	_ = main
}

func TestFxAppValidation(t *testing.T) {
	setGitLabOllamaEnv(t)

	logger := zap.NewNop()

	err := fx.ValidateApp(
		fx.Supply(logger),
		fx.Provide(
			newConfig,
			newVCSClient,
			newAIProvider,
			newReviewer,
		),
		fx.NopLogger,
	)
	if err != nil {
		t.Fatalf("fx.ValidateApp failed: %v", err)
	}
}

func TestNewConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setGitLabOllamaEnv(t)
		t.Setenv("COMMENT_PREFIX", "test-prefix")
		t.Setenv("RUN_TIMEOUT", "45s")
		t.Setenv("DELETE_BOT_COMMENTS", "false")

		cfg, err := newConfig()
		if err != nil {
			t.Fatalf("newConfig failed: %v", err)
		}

		if cfg.Runtime.CommentPrefix != "test-prefix" {
			t.Fatalf("expected comment prefix %q, got %q", "test-prefix", cfg.Runtime.CommentPrefix)
		}

		if cfg.Runtime.RunTimeout != 45*time.Second {
			t.Fatalf("expected run timeout %v, got %v", 45*time.Second, cfg.Runtime.RunTimeout)
		}

		if cfg.Runtime.DeleteBotComments {
			t.Fatal("expected delete bot comments to be false")
		}
	})

	t.Run("wraps parse error", func(t *testing.T) {
		t.Setenv("RUN_TIMEOUT", "not-a-duration")

		_, err := newConfig()
		if err == nil {
			t.Fatal("expected newConfig to fail")
		}

		if !strings.Contains(err.Error(), "create config: parse config") {
			t.Fatalf("expected wrapped config error, got %v", err)
		}
	})
}

func TestNewVCSClient(t *testing.T) {
	t.Parallel()

	t.Run("unsupported provider", func(t *testing.T) {
		t.Parallel()

		cfg := &domain.Config{VCS: domain.VCSConfig{Provider: "bitbucket"}}

		_, err := newVCSClient(cfg)
		if err == nil {
			t.Fatal("expected newVCSClient to fail")
		}

		if got := err.Error(); got != "unsupported VCS provider: bitbucket (supported: gitlab, github)" {
			t.Fatalf("unexpected error: %v", got)
		}
	})

	t.Run("github", func(t *testing.T) {
		t.Parallel()

		cfg := &domain.Config{
			Runtime: domain.RuntimeConfig{CommentPrefix: "test-prefix"},
			VCS: domain.VCSConfig{
				Provider: "github",
				GitHub: domain.GitHubConfig{
					Token:     "token",
					Owner:     "owner",
					Repo:      "repo",
					PRNumber:  "42",
					CommitSHA: "abc123",
				},
			},
		}

		client, err := newVCSClient(cfg)
		if err != nil {
			t.Fatalf("newVCSClient failed: %v", err)
		}

		if client == nil {
			t.Fatal("expected github client")
		}
	})

	t.Run("gitlab", func(t *testing.T) {
		t.Parallel()

		cfg := &domain.Config{
			Runtime: domain.RuntimeConfig{CommentPrefix: "test-prefix"},
			VCS: domain.VCSConfig{
				Provider: "gitlab",
				GitLab: domain.GitLabConfig{
					URL:                     "https://gitlab.example.com",
					Token:                   "token",
					ProjectID:               "123",
					MergeRequestIID:         "7",
					CommitSHA:               "abc123",
					MergeRequestDiffBaseSHA: "def456",
				},
			},
		}

		client, err := newVCSClient(cfg)
		if err != nil {
			t.Fatalf("newVCSClient failed: %v", err)
		}

		if client == nil {
			t.Fatal("expected gitlab client")
		}
	})
}

func TestNewGitHubClient(t *testing.T) {
	t.Parallel()

	t.Run("validates required settings", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			config  domain.GitHubConfig
			wantErr string
		}{
			{
				name:    "missing token",
				config:  domain.GitHubConfig{Owner: "owner", Repo: "repo", PRNumber: "1"},
				wantErr: "GITHUB_TOKEN is required for github provider",
			},
			{
				name:    "missing repo metadata",
				config:  domain.GitHubConfig{Token: "token", PRNumber: "1"},
				wantErr: "GITHUB_OWNER and GITHUB_REPO are required for github provider",
			},
			{
				name:    "missing pr number",
				config:  domain.GitHubConfig{Token: "token", Owner: "owner", Repo: "repo"},
				wantErr: "GITHUB_PR_NUMBER is required for github provider",
			},
		}

		for _, tt := range tests {

			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				cfg := &domain.Config{VCS: domain.VCSConfig{GitHub: tt.config}}

				_, err := newGitHubClient(cfg)
				if err == nil {
					t.Fatal("expected newGitHubClient to fail")
				}

				if got := err.Error(); got != tt.wantErr {
					t.Fatalf("unexpected error: %v", got)
				}
			})
		}
	})

	t.Run("wraps client creation error", func(t *testing.T) {
		t.Parallel()

		cfg := &domain.Config{
			Runtime: domain.RuntimeConfig{CommentPrefix: "test-prefix"},
			VCS: domain.VCSConfig{GitHub: domain.GitHubConfig{
				Token:    "token",
				Owner:    "owner",
				Repo:     "repo",
				PRNumber: "not-a-number",
			}},
		}

		_, err := newGitHubClient(cfg)
		if err == nil {
			t.Fatal("expected newGitHubClient to fail")
		}

		if !strings.Contains(err.Error(), "create GitHub client: parse PR number") {
			t.Fatalf("expected wrapped client creation error, got %v", err)
		}
	})
}

func TestNewGitLabClient(t *testing.T) {
	t.Parallel()

	t.Run("validates required settings", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			config  domain.GitLabConfig
			wantErr string
		}{
			{
				name:    "missing token",
				config:  domain.GitLabConfig{ProjectID: "123", MergeRequestIID: "1"},
				wantErr: "GITLAB_TOKEN is required for gitlab provider",
			},
			{
				name:    "missing project id",
				config:  domain.GitLabConfig{Token: "token", MergeRequestIID: "1"},
				wantErr: "CI_PROJECT_ID is required for gitlab provider",
			},
			{
				name:    "missing merge request iid",
				config:  domain.GitLabConfig{Token: "token", ProjectID: "123"},
				wantErr: "CI_MERGE_REQUEST_IID is required for gitlab provider",
			},
		}

		for _, tt := range tests {

			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				cfg := &domain.Config{VCS: domain.VCSConfig{GitLab: tt.config}}

				_, err := newGitLabClient(cfg)
				if err == nil {
					t.Fatal("expected newGitLabClient to fail")
				}

				if got := err.Error(); got != tt.wantErr {
					t.Fatalf("unexpected error: %v", got)
				}
			})
		}
	})

	t.Run("wraps iid parsing error", func(t *testing.T) {
		t.Parallel()

		cfg := &domain.Config{VCS: domain.VCSConfig{GitLab: domain.GitLabConfig{
			Token:           "token",
			ProjectID:       "123",
			MergeRequestIID: "not-a-number",
		}}}

		_, err := newGitLabClient(cfg)
		if err == nil {
			t.Fatal("expected newGitLabClient to fail")
		}

		if !strings.Contains(err.Error(), "parse CI_MERGE_REQUEST_IID") {
			t.Fatalf("expected parse error, got %v", err)
		}
	})
}

func TestNewAIProvider(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		cfg := &domain.Config{AI: domain.AIConfig{
			Provider: "ollama",
			Ollama: domain.OllamaConfig{
				URL:   "http://localhost:11434",
				Model: "llama3.2",
			},
		}}

		provider, err := newAIProvider(cfg)
		if err != nil {
			t.Fatalf("newAIProvider failed: %v", err)
		}

		if provider == nil {
			t.Fatal("expected AI provider")
		}
	})

	t.Run("wraps factory error", func(t *testing.T) {
		t.Parallel()

		cfg := &domain.Config{AI: domain.AIConfig{Provider: "unsupported"}}

		_, err := newAIProvider(cfg)
		if err == nil {
			t.Fatal("expected newAIProvider to fail")
		}

		if !strings.Contains(err.Error(), "create AI provider: unsupported AI provider") {
			t.Fatalf("expected wrapped AI provider error, got %v", err)
		}
	})
}

func TestNewReviewer(t *testing.T) {
	t.Parallel()

	cfg := &domain.Config{}
	mrProvider := mocks.NewMRProviderPort(t)
	aiProvider := mocks.NewAIProviderPort(t)
	logger := zap.NewNop()

	reviewer := newReviewer(cfg, mrProvider, aiProvider, logger)
	if reviewer == nil {
		t.Fatal("reviewer is nil")
	}
}

func TestRunReview(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mrProvider := mocks.NewMRProviderPort(t)
		aiProvider := mocks.NewAIProviderPort(t)
		logger := zap.NewNop()
		cfg := &domain.Config{Runtime: domain.RuntimeConfig{CommentPrefix: "prefix", RunTimeout: time.Second}}

		mrProvider.EXPECT().GetExistingComments(mock.Anything).Return(map[string][]string{}, nil).Once()
		mrProvider.EXPECT().GetMergeRequestChanges(mock.Anything).Return([]domain.Diff{{
			NewPath: "main.go",
			Content: "@@ -1 +1 @@\n-old\n+new",
		}}, nil).Once()
		aiProvider.EXPECT().ReviewCode(mock.Anything, mock.Anything).Return(`{"issues":[{"file":"main.go","severity":"warning","message":"fix this","line":3}]}`, nil).Once()
		mrProvider.EXPECT().AddMergeRequestDiscussion(mock.Anything, "main.go", 3, "prefix:**WARNING**: fix this").Return(nil).Once()

		reviewer := newReviewer(cfg, mrProvider, aiProvider, logger)

		if err := runReview(reviewer, logger, cfg); err != nil {
			t.Fatalf("runReview failed: %v", err)
		}
	})

	t.Run("wraps reviewer error", func(t *testing.T) {
		t.Parallel()

		mrProvider := mocks.NewMRProviderPort(t)
		aiProvider := mocks.NewAIProviderPort(t)
		logger := zap.NewNop()
		cfg := &domain.Config{Runtime: domain.RuntimeConfig{CommentPrefix: "prefix", RunTimeout: time.Second}}
		reviewErr := errors.New("boom")

		mrProvider.EXPECT().GetExistingComments(mock.Anything).Return(map[string][]string{}, nil).Once()
		mrProvider.EXPECT().GetMergeRequestChanges(mock.Anything).Return(nil, reviewErr).Once()

		reviewer := newReviewer(cfg, mrProvider, aiProvider, logger)

		err := runReview(reviewer, logger, cfg)
		if err == nil {
			t.Fatal("expected runReview to fail")
		}

		if !strings.Contains(err.Error(), "run review: get MR changes: boom") {
			t.Fatalf("expected wrapped review error, got %v", err)
		}
	})
}

func setGitLabOllamaEnv(t *testing.T) {
	t.Helper()

	t.Setenv("VCS_PROVIDER", "gitlab")
	t.Setenv("GITLAB_TOKEN", "test-token")
	t.Setenv("CI_PROJECT_ID", "123")
	t.Setenv("CI_MERGE_REQUEST_IID", "1")
	t.Setenv("CI_COMMIT_SHA", "abc123")
	t.Setenv("CI_MERGE_REQUEST_DIFF_BASE_SHA", "def456")
	t.Setenv("AI_PROVIDER", "ollama")
}

func TestRunReviewUsesConfiguredTimeout(t *testing.T) {
	t.Parallel()

	mrProvider := mocks.NewMRProviderPort(t)
	aiProvider := mocks.NewAIProviderPort(t)
	logger := zap.NewNop()
	cfg := &domain.Config{Runtime: domain.RuntimeConfig{RunTimeout: 50 * time.Millisecond}}

	mrProvider.EXPECT().GetExistingComments(mock.Anything).RunAndReturn(func(ctx context.Context) (map[string][]string, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected deadline on review context")
		}

		if time.Until(deadline) <= 0 {
			t.Fatal("expected future deadline on review context")
		}

		return map[string][]string{}, nil
	}).Once()
	mrProvider.EXPECT().GetMergeRequestChanges(mock.Anything).Return(nil, nil).Once()

	reviewer := newReviewer(cfg, mrProvider, aiProvider, logger)

	if err := runReview(reviewer, logger, cfg); err != nil {
		t.Fatalf("runReview failed: %v", err)
	}
}
