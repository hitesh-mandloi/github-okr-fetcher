package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github-okr-fetcher/internal/domain/entity"
)

// Repository implements the ConfigRepository interface
type Repository struct{}

// NewRepository creates a new config repository
func NewRepository() *Repository {
	return &Repository{}
}

// LoadConfig loads configuration from a file
func (r *Repository) LoadConfig(configPath string) (*entity.Config, error) {
	// Default config path if not provided
	if configPath == "" {
		configPath = "config.json"
	}
	
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}
	
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}
	
	var config entity.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}
	
	// Use configured default values if GitHub owner/repo not specified
	if config.GitHub.Owner == "" && config.Defaults.Organization != "" {
		config.GitHub.Owner = config.Defaults.Organization
	}
	if config.GitHub.Repo == "" && config.Defaults.Repository != "" {
		config.GitHub.Repo = config.Defaults.Repository
	}
	
	// Extract owner/repo from project URL only if still not specified
	if config.GitHub.ProjectURL != "" && config.GitHub.Owner == "" {
		if err := r.extractRepoInfo(&config); err != nil {
			return nil, fmt.Errorf("error extracting repository info from project URL: %v", err)
		}
	}
	
	return &config, nil
}

// GenerateExampleConfig generates an example configuration file
func (r *Repository) GenerateExampleConfig(filePath string) error {
	config := entity.Config{}
	config.GitHub.ProjectURL = "https://github.com/orgs/your-org/projects/123/views/456"
	config.Labels.Required = []string{"kind/okr", "team/your-team", "target/2026-q1"}
	config.Filter.Query = "label:\"target/2026-Q1\" label:\"kind/okr\" is:issue"
	config.Filter.UseSearch = true
	config.Output.Format = "markdown"
	config.Output.Title = "Your Team OKRs"
	config.Output.ProjectName = "Your Project Name"
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling example config: %v", err)
	}
	
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("error writing example config: %v", err)
	}
	
	return nil
}

// FindConfigFile looks for config files in standard locations
func (r *Repository) FindConfigFile() string {
	candidates := []string{
		"config.json",
		".github-okr-config.json",
		filepath.Join(os.Getenv("HOME"), ".github-okr-config.json"),
	}
	
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	
	return ""
}

// Helper methods

func (r *Repository) extractRepoInfo(config *entity.Config) error {
	if config.GitHub.ProjectURL == "" {
		return nil
	}
	
	// Extract from project URLs: https://github.com/orgs/org-name/projects/123/views/456
	// Extract from repo URLs: https://github.com/owner/repo/projects/123/views/456
	// Extract from issue URLs: https://github.com/owner/repo/issues/123
	patterns := []string{
		`https://github\.com/orgs/([^/]+)/projects/`,
		`https://github\.com/([^/]+)/([^/]+)/projects/`,
		`https://github\.com/([^/]+)/([^/]+)/issues/`,
		`https://github\.com/([^/]+)/([^/]+)/?$`,
	}
	
	for i, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(config.GitHub.ProjectURL)
		
		if len(matches) >= 2 {
			if i == 0 {
				// Organization project URL
				config.GitHub.Owner = matches[1]
				// Use configured default repo name for org projects
				if config.GitHub.Repo == "" {
					if config.Defaults.Repository != "" {
						config.GitHub.Repo = config.Defaults.Repository
					} else {
						config.GitHub.Repo = "repository"
					}
				}
			} else if len(matches) >= 3 {
				// Repository-based URL
				config.GitHub.Owner = matches[1]
				config.GitHub.Repo = matches[2]
			}
			return nil
		}
	}
	
	return fmt.Errorf("could not extract owner/repo from project URL: %s", config.GitHub.ProjectURL)
}