package service

import (
	"fmt"

	"github-okr-fetcher/internal/domain/entity"
	"github-okr-fetcher/internal/ports"
)

// ConfigService implements configuration management business logic
type ConfigService struct {
	configRepo ports.ConfigRepository
}

// NewConfigService creates a new configuration service
func NewConfigService(configRepo ports.ConfigRepository) *ConfigService {
	return &ConfigService{
		configRepo: configRepo,
	}
}

// GetConfig retrieves configuration with fallbacks and validation
func (s *ConfigService) GetConfig(configPath string) (*entity.Config, error) {
	// Find config file if path not provided
	if configPath == "" {
		configPath = s.configRepo.FindConfigFile()
	}
	
	// Load config
	config, err := s.configRepo.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	
	// Set defaults
	config = s.SetDefaults(config)
	
	// Validate
	if err := s.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return config, nil
}

// ValidateConfig validates configuration values
func (s *ConfigService) ValidateConfig(config *entity.Config) error {
	if config.GitHub.ProjectURL == "" && config.GitHub.Owner == "" {
		return fmt.Errorf("either github.project_url or github.owner is required")
	}
	
	// Additional validation can be added here
	return nil
}

// SetDefaults applies default values to configuration
func (s *ConfigService) SetDefaults(config *entity.Config) *entity.Config {
	if config.Output.Format == "" {
		config.Output.Format = "markdown"
	}
	
	if config.GitHub.Repo == "" {
		config.GitHub.Repo = "microservices"
	}
	
	if config.Performance.RateLimit == 0 {
		config.Performance.RateLimit = 5000
	}
	
	if config.Defaults.Organization == "" {
		config.Defaults.Organization = "kouzoh"
	}
	
	if config.Defaults.Repository == "" {
		config.Defaults.Repository = "microservices"
	}
	
	return config
}