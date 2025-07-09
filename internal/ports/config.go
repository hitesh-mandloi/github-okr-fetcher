package ports

import "github-okr-fetcher/internal/domain/entity"

// ConfigRepository defines the interface for configuration management
type ConfigRepository interface {
	LoadConfig(configPath string) (*entity.Config, error)
	GenerateExampleConfig(filePath string) error
	FindConfigFile() string
}

// ConfigService defines high-level configuration operations
type ConfigService interface {
	GetConfig(configPath string) (*entity.Config, error)
	ValidateConfig(config *entity.Config) error
	SetDefaults(config *entity.Config) *entity.Config
}