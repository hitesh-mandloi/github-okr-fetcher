package entity

import (
	"fmt"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	GitHub          GitHubConfig           `json:"github"`
	Labels          LabelsConfig           `json:"labels"`
	Filter          FilterConfig           `json:"filter"`
	Output          OutputConfig           `json:"output"`
	Performance     PerformanceConfig      `json:"performance"`
	Defaults        DefaultsConfig         `json:"default_values"`
	LiteLLM         LiteLLMConfig          `json:"litellm"`
	Cache           CacheConfig            `json:"cache"`
	Patterns        PatternsConfig         `json:"patterns"`
	StatusDetection StatusDetectionConfig  `json:"status_detection"`
}

// GitHubConfig contains GitHub-related configuration
// Note: GitHub token must be provided via GITHUB_TOKEN environment variable
type GitHubConfig struct {
	ProjectURL    string `json:"project_url"`
	Owner         string `json:"owner,omitempty"`
	Repo          string `json:"repo,omitempty"`
	TimeoutSec    int    `json:"timeout_seconds,omitempty"`
	RateLimit     int    `json:"rate_limit_per_hour,omitempty"`
	MaxRetries    int    `json:"max_retries,omitempty"`
	PageSize      int    `json:"page_size,omitempty"`
	MaxIssuesLimit int   `json:"max_issues_limit,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
}

// LabelsConfig contains label filtering configuration
type LabelsConfig struct {
	Required []string `json:"required"`
}

// FilterConfig contains issue filtering configuration
type FilterConfig struct {
	Query     string `json:"query,omitempty"`
	UseSearch bool   `json:"use_search,omitempty"`
}

// OutputConfig contains output formatting configuration
type OutputConfig struct {
	Format            string           `json:"format"` // "markdown", "json", or "google-docs"
	File              string           `json:"file"`
	Title             string           `json:"title,omitempty"`
	ProjectName       string           `json:"project_name,omitempty"`
	FilenamePattern   string           `json:"filename_pattern,omitempty"`
	TimestampFormat   string           `json:"timestamp_format,omitempty"`
	ProgressBarSegs   int              `json:"progress_bar_segments,omitempty"`
	GoogleDocs        GoogleDocsConfig `json:"google_docs"`
}

// GoogleDocsConfig contains Google Docs integration configuration
// Note: OAuth credentials must be provided via GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables
type GoogleDocsConfig struct {
	URL          string `json:"url"`
}

// PerformanceConfig contains performance-related settings
type PerformanceConfig struct {
	MaxConcurrency int  `json:"max_concurrency,omitempty"`
	RateLimit      int  `json:"rate_limit_per_hour,omitempty"`
	CacheEnabled   bool `json:"cache_enabled,omitempty"`
}

// DefaultsConfig contains default values
type DefaultsConfig struct {
	Organization string `json:"organization,omitempty"`
	Repository   string `json:"repository,omitempty"`
}

// LiteLLMConfig contains LiteLLM API configuration for OKR analysis
// Note: LiteLLM API token must be provided via LITELLM_TOKEN environment variable
type LiteLLMConfig struct {
	BaseURL         string `json:"base_url"`
	Model           string `json:"model"`
	Enabled         bool   `json:"enabled"`
	TimeoutSec      int    `json:"timeout_seconds,omitempty"`
	WordLimit       int    `json:"analysis_word_limit,omitempty"`
}

// GetLabels returns the required labels with whitespace trimmed
func (c *Config) GetLabels() []string {
	var labels []string
	for _, label := range c.Labels.Required {
		if trimmed := strings.TrimSpace(label); trimmed != "" {
			labels = append(labels, trimmed)
		}
	}
	return labels
}

// ShouldUseSearch returns true if search-based filtering should be used
func (c *Config) ShouldUseSearch() bool {
	return c.Filter.UseSearch || c.Filter.Query != ""
}

// GetSearchQuery builds a search query from configuration
func (c *Config) GetSearchQuery() string {
	if c.Filter.Query != "" {
		return c.Filter.Query
	}
	
	if len(c.Labels.Required) > 0 {
		var parts []string
		for _, label := range c.Labels.Required {
			parts = append(parts, fmt.Sprintf("label:\"%s\"", label))
		}
		parts = append(parts, "is:issue")
		return strings.Join(parts, " ")
	}
	
	return "is:issue"
}

// GetOutputFile generates an output filename
func (c *Config) GetOutputFile(owner string, projectID, viewID int) string {
	if c.Output.File != "" {
		return c.Output.File
	}
	
	ext := ".md"
	if c.Output.Format == "json" {
		ext = ".json"
	} else if c.Output.Format == "google-docs" {
		ext = ".txt"
	}
	
	timestampFormat := "20060102_150405"
	if c.Output.TimestampFormat != "" {
		timestampFormat = c.Output.TimestampFormat
	}
	timestamp := time.Now().Format(timestampFormat)
	
	filenamePattern := "okr-report_%s_%d_%d_%s%s"
	if c.Output.FilenamePattern != "" {
		filenamePattern = c.Output.FilenamePattern
	}
	
	if viewID > 0 {
		return fmt.Sprintf(filenamePattern, owner, projectID, viewID, timestamp, ext)
	}
	// Adjust pattern for projects without view ID
	simplePattern := strings.Replace(filenamePattern, "_%d_%d_", "_%d_", 1)
	return fmt.Sprintf(simplePattern, owner, projectID, timestamp, ext)
}

// CacheConfig contains caching configuration
type CacheConfig struct {
	Enabled         bool `json:"enabled,omitempty"`
	IssuesTTLMin   int  `json:"issues_ttl_minutes,omitempty"`
	CommentsTTLMin int  `json:"comments_ttl_minutes,omitempty"`
	GraphQLTTLMin  int  `json:"graphql_ttl_minutes,omitempty"`
}

// PatternsConfig contains regex patterns for detection
type PatternsConfig struct {
	WeeklyUpdateRegex   string   `json:"weekly_update_regex,omitempty"`
	ParentIssuePatterns []string `json:"parent_issue_patterns,omitempty"`
}

// StatusDetectionConfig contains keywords for status detection
type StatusDetectionConfig struct {
	CompletedKeywords []string `json:"completed_keywords,omitempty"`
	BlockedKeywords   []string `json:"blocked_keywords,omitempty"`
	AtRiskKeywords    []string `json:"at_risk_keywords,omitempty"`
	OnTrackKeywords   []string `json:"on_track_keywords,omitempty"`
}