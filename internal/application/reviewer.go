package application

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/adlandh/ai-mr-reviewer/internal/domain"
	"go.uber.org/zap"
)

type Reviewer struct {
	logger     *zap.Logger
	mrProvider domain.MRProviderPort
	aiProvider domain.AIProviderPort
	runtime    domain.RuntimeConfig
}

type reviewResponse struct {
	Issues []reviewIssuePayload `json:"issues"`
}

type reviewIssuePayload struct {
	FilePath string `json:"file"`
	Path     string `json:"path"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Line     int    `json:"line"`
}

type reviewBatch struct {
	content string
	diffs   []domain.Diff
}

var languageMap = map[string]string{
	".go":    "Go",
	".js":    "JavaScript/TypeScript",
	".jsx":   "JavaScript/TypeScript",
	".ts":    "JavaScript/TypeScript",
	".tsx":   "JavaScript/TypeScript",
	".py":    "Python",
	".java":  "Java",
	".rs":    "Rust",
	".c":     "C",
	".h":     "C",
	".cpp":   "C++",
	".hpp":   "C++",
	".cc":    "C++",
	".rb":    "Ruby",
	".php":   "PHP",
	".swift": "Swift",
	".kt":    "Kotlin",
	".kts":   "Kotlin",
	".scala": "Scala",
	".sh":    "Shell",
	".bash":  "Shell",
	".sql":   "SQL",
	".yaml":  "YAML",
	".yml":   "YAML",
	".json":  "JSON",
	".xml":   "XML",
	".md":    "Markdown",
}

func NewReviewer(runtime domain.RuntimeConfig, mrProvider domain.MRProviderPort, aiProvider domain.AIProviderPort, logger *zap.Logger) *Reviewer {
	return &Reviewer{runtime: runtime, mrProvider: mrProvider, aiProvider: aiProvider, logger: logger}
}

func (r *Reviewer) Run(ctx context.Context) error {
	if r.runtime.DeleteBotComments {
		if err := r.mrProvider.DeleteBotCommentsExceptResolved(ctx); err != nil {
			r.logger.Warn("cannot delete bot comments", zap.Error(err))
		}
	}

	existing, err := r.mrProvider.GetExistingComments(ctx)
	if err != nil {
		r.logger.Warn("cannot read existing comments", zap.Error(err))

		existing = map[string][]string{}
	}

	diffs, err := r.mrProvider.GetMergeRequestChanges(ctx)
	if err != nil {
		return fmt.Errorf("get MR changes: %w", err)
	}

	prefix := r.runtime.CommentPrefix + ":"

	filteredDiffs := r.filterNewDiffs(diffs, existing, prefix)
	if len(filteredDiffs) == 0 {
		return nil
	}

	if err := r.reviewDiffs(ctx, filteredDiffs); err != nil {
		return fmt.Errorf("review diffs: %w", err)
	}

	return nil
}

func (r *Reviewer) filterNewDiffs(diffs []domain.Diff, existing map[string][]string, prefix string) []domain.Diff {
	filtered := make([]domain.Diff, 0, len(diffs))

	for _, d := range diffs {
		if !hasExistingComments(d.NewPath, existing, prefix) {
			filtered = append(filtered, d)
		}
	}

	return filtered
}

func hasExistingComments(path string, existing map[string][]string, prefix string) bool {
	for key, bodies := range existing {
		if strings.HasPrefix(key, path+":") {
			for _, body := range bodies {
				if strings.HasPrefix(body, prefix) {
					return true
				}
			}
		}
	}

	return false
}

func (r *Reviewer) reviewDiffs(ctx context.Context, diffs []domain.Diff) error {
	batches := buildReviewBatches(diffs, r.runtime.MaxDiffBytes)
	for i, batch := range batches {
		if err := r.reviewBatch(ctx, batch); err != nil {
			return fmt.Errorf("review batch %d/%d: %w", i+1, len(batches), err)
		}
	}

	return nil
}

func (r *Reviewer) reviewBatch(ctx context.Context, batch reviewBatch) error {
	reviewText, err := r.aiProvider.ReviewCode(ctx, batch.content)
	if err != nil {
		return fmt.Errorf("review code: %w", err)
	}

	issues, err := parseReviewResponse(reviewText)
	if err != nil {
		return fmt.Errorf("parse review response: %w", err)
	}

	knownFiles := make(map[string]struct{}, len(batch.diffs))
	for _, d := range batch.diffs {
		knownFiles[d.NewPath] = struct{}{}
	}

	prefix := r.runtime.CommentPrefix

	for _, issue := range issues {
		validated, reason := validateReviewIssue(issue, knownFiles)
		if reason != "" {
			r.logger.Warn("skip invalid issue", zap.String("reason", reason), zap.String("file", issue.FilePath), zap.Int("line", issue.Line))
			continue
		}

		body := fmt.Sprintf("%s:**%s**: %s", prefix, strings.ToUpper(validated.Severity), validated.Message)
		if err := r.mrProvider.AddMergeRequestDiscussion(ctx, validated.FilePath, validated.Line, body); err != nil {
			r.logger.Warn("failed to add comment", zap.String("path", validated.FilePath), zap.Int("line", validated.Line), zap.Error(err))
		}
	}

	return nil
}

func validateReviewIssue(issue domain.ReviewIssue, knownFiles map[string]struct{}) (domain.ReviewIssue, string) {
	if issue.FilePath == "" && len(knownFiles) == 1 {
		for issue.FilePath = range knownFiles {
		}
	}

	if _, ok := knownFiles[issue.FilePath]; !ok {
		return issue, "unknown file"
	}

	if issue.Line <= 0 {
		return issue, "line must be positive"
	}

	issue.Severity = strings.ToLower(strings.TrimSpace(issue.Severity))
	switch issue.Severity {
	case "error", "warning", "info":
	default:
		return issue, "unsupported severity"
	}

	issue.Message = strings.TrimSpace(issue.Message)
	if issue.Message == "" {
		return issue, "message must not be blank"
	}

	return issue, ""
}

func parseReviewResponse(response string) ([]domain.ReviewIssue, error) {
	trimmed := strings.TrimSpace(response)

	jsonStr := extractJSON(trimmed)
	if jsonStr == "" {
		return nil, nil
	}

	var parsed reviewResponse
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	issues := make([]domain.ReviewIssue, 0, len(parsed.Issues))
	for _, issue := range parsed.Issues {
		filePath := issue.FilePath
		if filePath == "" {
			filePath = issue.Path
		}

		issues = append(issues, domain.ReviewIssue{
			FilePath: filePath,
			Severity: issue.Severity,
			Message:  issue.Message,
			Line:     issue.Line,
		})
	}

	return issues, nil
}

func buildReviewBatches(diffs []domain.Diff, maxBytes int) []reviewBatch {
	sortedDiffs := slices.Clone(diffs)
	slices.SortStableFunc(sortedDiffs, func(a, b domain.Diff) int {
		return strings.Compare(a.NewPath, b.NewPath)
	})

	var (
		batches    []reviewBatch
		batchDiffs []domain.Diff
		sections   []string
	)

	batchBytes := 0

	for _, d := range sortedDiffs {
		section := renderDiffSection(d)

		sectionBytes := len(section)
		if len(sections) > 0 {
			sectionBytes += 2
		}

		if maxBytes > 0 && len(sections) > 0 && batchBytes+sectionBytes > maxBytes {
			batches = append(batches, reviewBatch{diffs: slices.Clone(batchDiffs), content: strings.Join(sections, "\n\n")})
			batchDiffs = batchDiffs[:0]
			sections = sections[:0]
			batchBytes = 0
			sectionBytes = len(section)
		}

		batchDiffs = append(batchDiffs, d)
		sections = append(sections, section)
		batchBytes += sectionBytes
	}

	if len(sections) > 0 {
		batches = append(batches, reviewBatch{diffs: slices.Clone(batchDiffs), content: strings.Join(sections, "\n\n")})
	}

	return batches
}

func renderDiffSection(d domain.Diff) string {
	return "File: " + d.NewPath +
		"\nLanguage: " + detectLanguage(d.NewPath) +
		"\nDiff:\n" + d.Content
}

func extractJSON(s string) string {
	if idx := strings.Index(s, "```json"); idx != -1 {
		end := strings.Index(s[idx+7:], "```")
		if end != -1 {
			return s[idx+7 : idx+7+end]
		}
	}

	if _, after, ok := strings.Cut(s, "```"); ok {
		content := after
		if before, _, ok := strings.Cut(content, "```"); ok {
			extracted := strings.TrimSpace(before)
			if isJSON(extracted) {
				return extracted
			}
		}
	}

	start := findJSONStart(s)
	if start == -1 {
		return ""
	}

	end := findMatchingBracket(s, start)
	if end == -1 {
		return ""
	}

	return s[start : end+1]
}

func findJSONStart(s string) int {
	openBrace := strings.Index(s, "{")
	openBracket := strings.Index(s, "[")

	switch {
	case openBrace != -1 && openBracket != -1:
		return min(openBrace, openBracket)
	case openBrace != -1:
		return openBrace
	case openBracket != -1:
		return openBracket
	default:
		return -1
	}
}

func isJSON(s string) bool {
	s = strings.TrimSpace(s)

	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

func findMatchingBracket(s string, start int) int {
	openChar := rune(s[start])
	closeChar := openChar

	if openChar == '{' {
		closeChar = '}'
	}

	count := 0

	for i, r := range s[start:] {
		switch r {
		case openChar:
			count++
		case closeChar:
			count--
			if count == 0 {
				return start + i
			}
		}
	}

	return -1
}

func detectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if lang, ok := languageMap[ext]; ok {
		return lang
	}

	return "Unknown"
}
