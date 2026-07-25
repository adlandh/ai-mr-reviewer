package config

import (
	"testing"
	"time"
)

func TestNewConfigWithDefaults(t *testing.T) {
	t.Setenv("VCS_PROVIDER", "gitlab")
	t.Setenv("GITLAB_TOKEN", "token")
	t.Setenv("CI_PROJECT_ID", "123")
	t.Setenv("CI_MERGE_REQUEST_IID", "1")
	t.Setenv("CI_COMMIT_SHA", "abc123")
	t.Setenv("CI_MERGE_REQUEST_DIFF_BASE_SHA", "def456")
	t.Setenv("CI_SERVER_URL", "")
	t.Setenv("OLLAMA_URL", "")
	t.Setenv("OLLAMA_MODEL", "")

	cfg, err := New()
	if err != nil {
		t.Fatalf("expected config, got error: %v", err)
	}

	if cfg.VCS.Provider != "gitlab" {
		t.Fatalf("unexpected VCS provider: %s", cfg.VCS.Provider)
	}
	if cfg.VCS.GitLab.URL != "https://gitlab.com" {
		t.Fatalf("unexpected GitLab URL: %s", cfg.VCS.GitLab.URL)
	}
	if cfg.AI.Ollama.URL != "http://localhost:11434" {
		t.Fatalf("unexpected Ollama URL: %s", cfg.AI.Ollama.URL)
	}
	if cfg.AI.Ollama.Model != "llama3.2" {
		t.Fatalf("unexpected Ollama model: %s", cfg.AI.Ollama.Model)
	}
	if cfg.VCS.GitLab.MergeRequestIID != "1" {
		t.Fatalf("unexpected MergeRequestIID: %s", cfg.VCS.GitLab.MergeRequestIID)
	}
	if cfg.VCS.GitLab.CommitSHA != "abc123" {
		t.Fatalf("unexpected CommitSHA: %s", cfg.VCS.GitLab.CommitSHA)
	}
	if cfg.VCS.GitLab.MergeRequestDiffBaseSHA != "def456" {
		t.Fatalf("unexpected MergeRequestDiffBaseSHA: %s", cfg.VCS.GitLab.MergeRequestDiffBaseSHA)
	}
	if cfg.Runtime.RunTimeout != 10*time.Minute {
		t.Fatalf("unexpected RunTimeout: %s", cfg.Runtime.RunTimeout)
	}
	if cfg.Runtime.MaxDiffBytes != 100000 {
		t.Fatalf("unexpected MaxDiffBytes: %d", cfg.Runtime.MaxDiffBytes)
	}
}

func TestNewConfigMaxDiffBytesOverride(t *testing.T) {
	t.Setenv("MAX_DIFF_BYTES", "2048")

	cfg, err := New()
	if err != nil {
		t.Fatalf("expected config, got error: %v", err)
	}
	if cfg.Runtime.MaxDiffBytes != 2048 {
		t.Fatalf("unexpected MaxDiffBytes: %d", cfg.Runtime.MaxDiffBytes)
	}
}

func TestNewConfigRejectsInvalidMaxDiffBytes(t *testing.T) {
	for _, value := range []string{"0", "-1", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("MAX_DIFF_BYTES", value)

			if _, err := New(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestNewConfigMissingRequired(t *testing.T) {
	t.Setenv("VCS_PROVIDER", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_PROJECT_ID", "")
	t.Setenv("CI_MERGE_REQUEST_IID", "")
	t.Setenv("CI_COMMIT_SHA", "")
	t.Setenv("CI_MERGE_REQUEST_DIFF_BASE_SHA", "")

	cfg, err := New()
	if err != nil {
		t.Fatalf("expected config without error, got: %v", err)
	}

	if cfg.VCS.Provider != "gitlab" {
		t.Fatalf("unexpected VCS provider: %s", cfg.VCS.Provider)
	}
}
