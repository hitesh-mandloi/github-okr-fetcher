package service

import (
	"encoding/json"
	"fmt"
	"os"

	"github-okr-fetcher/internal/domain/entity"
	"github-okr-fetcher/internal/ports"
)

// AnalysisService handles OKR analysis using LiteLLM
type AnalysisService struct {
	analysisClient ports.AnalysisService
	config         *entity.Config
}

// NewAnalysisService creates a new analysis service
func NewAnalysisService(analysisClient ports.AnalysisService, config *entity.Config) *AnalysisService {
	return &AnalysisService{
		analysisClient: analysisClient,
		config:         config,
	}
}

// AnalysisResult represents the result of OKR analysis
type AnalysisResult struct {
	Analysis string `json:"analysis"`
	Enabled  bool   `json:"enabled"`
}

// AnalyzeProject analyzes a project's OKRs and returns insights
func (s *AnalysisService) AnalyzeProject(project *entity.Project) (*AnalysisResult, error) {
	// Check if LiteLLM is enabled and token is available in environment
	liteLLMToken := os.Getenv("LITELLM_TOKEN")
	if !s.config.LiteLLM.Enabled || liteLLMToken == "" {
		return &AnalysisResult{
			Analysis: "",
			Enabled:  false,
		}, nil
	}

	// Convert project data to JSON for analysis
	projectData, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal project data: %w", err)
	}

	// Get analysis from LiteLLM
	analysis, err := s.analysisClient.AnalyzeOKRs(string(projectData))
	if err != nil {
		return nil, fmt.Errorf("failed to analyze OKRs: %w", err)
	}

	return &AnalysisResult{
		Analysis: analysis,
		Enabled:  true,
	}, nil
}