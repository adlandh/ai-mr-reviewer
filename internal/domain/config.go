package domain

import "time"

type Config struct {
	AI      AIConfig
	VCS     VCSConfig
	Runtime RuntimeConfig
}

type RuntimeConfig struct {
	CommentPrefix     string
	RunTimeout        time.Duration
	DeleteBotComments bool
	MaxDiffBytes      int
}

type VCSConfig struct {
	Provider string
	GitHub   GitHubConfig
	GitLab   GitLabConfig
}

type GitHubConfig struct {
	Token     string
	Owner     string
	Repo      string
	PRNumber  string
	CommitSHA string
}

type GitLabConfig struct {
	URL                     string
	Token                   string
	ProjectID               string
	MergeRequestIID         string
	CommitSHA               string
	MergeRequestDiffBaseSHA string
}

type AIConfig struct {
	Provider  string
	Ollama    OllamaConfig
	OpenAI    OpenAIConfig
	Anthropic AnthropicConfig
	Copilot   CopilotConfig
}

type OllamaConfig struct {
	URL    string
	APIKey string
	Model  string
}

type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

type AnthropicConfig struct {
	AuthToken string
	BaseURL   string
	Model     string
}

type CopilotConfig struct {
	Token   string
	BaseURL string
	Model   string
}
