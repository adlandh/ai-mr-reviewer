package config

import (
	"fmt"
	"time"

	"github.com/adlandh/ai-mr-reviewer/internal/domain"
	"github.com/caarlos0/env/v11"
)

type envConfig struct {
	GitHubPRNumber          string        `env:"GITHUB_PR_NUMBER"`
	AIProvider              string        `env:"AI_PROVIDER,notEmpty" envDefault:"ollama"`
	GitLabToken             string        `env:"GITLAB_TOKEN"`
	ProjectID               string        `env:"CI_PROJECT_ID"`
	MergeRequestIID         string        `env:"CI_MERGE_REQUEST_IID"`
	CommitSHA               string        `env:"CI_COMMIT_SHA"`
	MergeRequestDiffBaseSHA string        `env:"CI_MERGE_REQUEST_DIFF_BASE_SHA"`
	GitHubToken             string        `env:"GITHUB_TOKEN"`
	GitHubOwner             string        `env:"GITHUB_OWNER"`
	OllamaURL               string        `env:"OLLAMA_URL" envDefault:"http://localhost:11434"`
	OllamaAPIKey            string        `env:"OLLAMA_API_KEY"`
	GitLabURL               string        `env:"CI_SERVER_URL" envDefault:"https://gitlab.com"`
	VCSProvider             string        `env:"VCS_PROVIDER,notEmpty" envDefault:"gitlab"`
	GitHubRepo              string        `env:"GITHUB_REPO"`
	OllamaModel             string        `env:"OLLAMA_MODEL" envDefault:"llama3.2"`
	OpenAIAPIKey            string        `env:"OPENAI_API_KEY"`
	OpenAIBaseURL           string        `env:"OPENAI_BASE_URL" envDefault:"https://api.openai.com/v1"`
	OpenAIModel             string        `env:"OPENAI_MODEL" envDefault:"GPT-5.2-Codex"`
	AnthropicAuthToken      string        `env:"ANTHROPIC_AUTH_TOKEN"`
	AnthropicBaseURL        string        `env:"ANTHROPIC_BASE_URL" envDefault:"https://api.anthropic.com/v1/"`
	AnthropicModel          string        `env:"ANTHROPIC_MODEL" envDefault:"claude-sonnet-4-20250514"`
	CopilotBaseURL          string        `env:"COPILOT_BASE_URL" envDefault:"https://models.github.ai/inference"`
	CopilotModel            string        `env:"COPILOT_MODEL" envDefault:"openai/gpt-4.1"`
	CommentPrefix           string        `env:"COMMENT_PREFIX,notEmpty" envDefault:"ai-mr-reviewer"`
	RunTimeout              time.Duration `env:"RUN_TIMEOUT" envDefault:"10m"`
	DeleteBotComments       bool          `env:"DELETE_BOT_COMMENTS" envDefault:"true"`
	MaxDiffBytes            int           `env:"MAX_DIFF_BYTES" envDefault:"100000"`
}

func New() (*domain.Config, error) {
	raw, err := env.ParseAsWithOptions[envConfig](env.Options{RequiredIfNoDef: false})
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if raw.MaxDiffBytes <= 0 {
		return nil, fmt.Errorf("MAX_DIFF_BYTES must be positive")
	}

	return &domain.Config{
		Runtime: domain.RuntimeConfig{
			CommentPrefix:     raw.CommentPrefix,
			RunTimeout:        raw.RunTimeout,
			DeleteBotComments: raw.DeleteBotComments,
			MaxDiffBytes:      raw.MaxDiffBytes,
		},
		VCS: domain.VCSConfig{
			Provider: raw.VCSProvider,
			GitHub: domain.GitHubConfig{
				Token:     raw.GitHubToken,
				Owner:     raw.GitHubOwner,
				Repo:      raw.GitHubRepo,
				PRNumber:  raw.GitHubPRNumber,
				CommitSHA: raw.CommitSHA,
			},
			GitLab: domain.GitLabConfig{
				URL:                     raw.GitLabURL,
				Token:                   raw.GitLabToken,
				ProjectID:               raw.ProjectID,
				MergeRequestIID:         raw.MergeRequestIID,
				CommitSHA:               raw.CommitSHA,
				MergeRequestDiffBaseSHA: raw.MergeRequestDiffBaseSHA,
			},
		},
		AI: domain.AIConfig{
			Provider: raw.AIProvider,
			Ollama: domain.OllamaConfig{
				URL:    raw.OllamaURL,
				APIKey: raw.OllamaAPIKey,
				Model:  raw.OllamaModel,
			},
			OpenAI: domain.OpenAIConfig{
				APIKey:  raw.OpenAIAPIKey,
				BaseURL: raw.OpenAIBaseURL,
				Model:   raw.OpenAIModel,
			},
			Anthropic: domain.AnthropicConfig{
				AuthToken: raw.AnthropicAuthToken,
				BaseURL:   raw.AnthropicBaseURL,
				Model:     raw.AnthropicModel,
			},
			Copilot: domain.CopilotConfig{
				Token:   raw.GitHubToken,
				BaseURL: raw.CopilotBaseURL,
				Model:   raw.CopilotModel,
			},
		},
	}, nil
}
