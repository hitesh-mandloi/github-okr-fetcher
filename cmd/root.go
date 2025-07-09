package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github-okr-fetcher/internal/adapters/config"
	"github-okr-fetcher/internal/adapters/github"
	"github-okr-fetcher/internal/adapters/litellm"
	"github-okr-fetcher/internal/adapters/output"
	"github-okr-fetcher/internal/domain/entity"
	"github-okr-fetcher/internal/domain/service"
	"github-okr-fetcher/internal/ports"
)

var (
	projectURL       string
	outputFile       string
	jsonOutput       bool
	googleDocsOutput bool
	skipLabelFilter  bool
	customLabels     string
	configFile       string
)

var rootCmd = &cobra.Command{
	Use:   "github-okr-fetcher",
	Short: "Fetch GitHub issues and their weekly update comments from GitHub project views",
	Long: `GitHub OKR Fetcher is a configuration-driven tool that fetches GitHub issues 
and their weekly update comments from GitHub project views.

Features:
- Configuration-driven: JSON config file support for tokens, URLs, and labels
- Smart filtering: AND condition for multiple required labels
- Search-based queries: GitHub search API integration for efficient label filtering
- Parent-child relationships: Uses explicit "Parent Issue:" references for OKR hierarchy
- Weekly updates: Extracts and displays latest "weekly update yyyy-mm-dd" comments
- Progress tracking: Visual progress bars and completion metrics
- Multiple output formats: Markdown reports and JSON data export`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMain()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&projectURL, "url", "u", "", "GitHub project view URL (overrides config)")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (overrides config, default: auto-generated)")
	rootCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output JSON instead of Markdown (overrides config)")
	rootCmd.Flags().BoolVar(&googleDocsOutput, "google-docs", false, "Output Google Docs compatible plain text format")
	rootCmd.Flags().BoolVar(&skipLabelFilter, "skip-labels", false, "Skip label filtering and process all issues")
	rootCmd.Flags().StringVarP(&customLabels, "labels", "l", "", "Comma-separated list of required labels (overrides config)")
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "Config file path (default: config.json)")
}

func runMain() error {
	// Initialize repositories and services
	configRepo := config.NewRepository()
	configService := service.NewConfigService(configRepo)

	// Load configuration
	var appConfig *entity.Config
	var err error

	if configFile == "" {
		configFile = configRepo.FindConfigFile()
	}

	if configFile != "" {
		appConfig, err = configService.GetConfig(configFile)
		if err != nil {
			fmt.Printf("Warning: Could not load config file '%s': %v\n", configFile, err)
			fmt.Println("Falling back to command line arguments and environment variables")
		} else {
			fmt.Printf("✅ Loaded config from: %s\n", configFile)
		}
	}

	// Apply command line overrides
	if appConfig == nil {
		appConfig = &entity.Config{}
		appConfig = configService.SetDefaults(appConfig)
	}

	// GitHub token: environment variable only for security
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GitHub token required. Set GITHUB_TOKEN environment variable")
	}

	// Project URL: CLI flag > config file
	if projectURL != "" {
		appConfig.GitHub.ProjectURL = projectURL
	}
	if appConfig.GitHub.ProjectURL == "" {
		return fmt.Errorf("GitHub project URL required. Use -url flag or provide in config file")
	}

	// Labels: CLI flag > config file
	if customLabels != "" {
		appConfig.Labels.Required = strings.Split(customLabels, ",")
		for i, label := range appConfig.Labels.Required {
			appConfig.Labels.Required[i] = strings.TrimSpace(label)
		}
	}

	// Apply skip label filter
	if skipLabelFilter {
		appConfig.Labels.Required = nil
	}

	// Output format: CLI flag > config file
	if jsonOutput {
		appConfig.Output.Format = "json"
	} else if googleDocsOutput {
		appConfig.Output.Format = "google-docs"
	}

	// Initialize GitHub repository and service
	githubRepo := github.NewRepository(token, appConfig)
	okrService := service.NewOKRService(githubRepo)

	// Initialize LiteLLM analysis service if enabled
	// Get LiteLLM token from environment variable for security
	liteLLMToken := os.Getenv("LITELLM_TOKEN")
	var analysisService *service.AnalysisService
	if appConfig.LiteLLM.Enabled && liteLLMToken != "" {
		// Pass token via parameter instead of config for security
		liteLLMClient := litellm.NewClient(appConfig.LiteLLM, liteLLMToken)
		analysisService = service.NewAnalysisService(liteLLMClient, appConfig)
		fmt.Printf("🤖 LiteLLM analysis enabled with model: %s\n", appConfig.LiteLLM.Model)
	}

	// Initialize output service
	reportGenerator := output.NewReportGeneratorWithConfig(appConfig)

	// Main application logic
	ctx := context.Background()

	fmt.Printf("🚀 Starting OKR data collection...\n")

	// Fetch and process OKR data
	objectives, projectInfo, err := okrService.FetchOKRData(ctx, appConfig)
	if err != nil {
		return fmt.Errorf("error fetching OKR data: %v", err)
	}

	// Perform LiteLLM analysis if enabled
	var analysisResult *service.AnalysisResult
	if analysisService != nil {
		fmt.Printf("🔍 Analyzing OKR data with AI...\n")
		
		// Create a project entity for analysis
		project := &entity.Project{
			Info:       projectInfo,
			Objectives: objectives,
		}
		
		analysisResult, err = analysisService.AnalyzeProject(project)
		if err != nil {
			fmt.Printf("⚠️ Warning: AI analysis failed: %v\n", err)
			analysisResult = &service.AnalysisResult{Analysis: "", Enabled: false}
		} else if analysisResult.Enabled {
			fmt.Printf("✅ AI analysis completed successfully\n")
		}
	}

	// Determine output format
	var outputFormat ports.OutputFormat
	switch appConfig.Output.Format {
	case "json":
		outputFormat = ports.OutputFormatJSON
	case "google-docs":
		outputFormat = ports.OutputFormatGoogleDocs
	default:
		outputFormat = ports.OutputFormatMarkdown
	}

	// Determine output filename
	if outputFile == "" {
		outputFile = appConfig.GetOutputFile(projectInfo.Owner, projectInfo.ProjectID, projectInfo.ViewID)
		// Override extension if CLI flag was used
		if jsonOutput {
			timestamp := time.Now().Format("20060102_150405")
			outputFile = fmt.Sprintf("okr-report_%s_%d_%d_%s.json", projectInfo.Owner, projectInfo.ProjectID, projectInfo.ViewID, timestamp)
		} else if googleDocsOutput {
			timestamp := time.Now().Format("20060102_150405")
			outputFile = fmt.Sprintf("okr-report_%s_%d_%d_%s.txt", projectInfo.Owner, projectInfo.ProjectID, projectInfo.ViewID, timestamp)
		}
	}

	// Check for Google Docs direct integration
	// Get Google OAuth credentials from environment variables for security
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	
	if outputFormat == ports.OutputFormatGoogleDocs &&
		appConfig.Output.GoogleDocs.URL != "" &&
		googleClientID != "" &&
		googleClientSecret != "" {

		fmt.Printf("🔗 Google Docs integration enabled, writing directly to document...\n")

		// Generate report with Google Docs integration (with AI analysis if available)
		if analysisResult != nil && analysisResult.Enabled {
			err = reportGenerator.GenerateReportWithGoogleDocsAndAnalysis(
				objectives,
				projectInfo,
				outputFormat,
				outputFile,
				appConfig.Output.GoogleDocs.URL,
				googleClientID,
				googleClientSecret,
				analysisResult.Analysis,
			)
		} else {
			err = reportGenerator.GenerateReportWithGoogleDocs(
				objectives,
				projectInfo,
				outputFormat,
				outputFile,
				appConfig.Output.GoogleDocs.URL,
				googleClientID,
				googleClientSecret,
			)
		}
		if err != nil {
			return fmt.Errorf("error writing to Google Docs: %v", err)
		}

		fmt.Printf("✅ Report written directly to Google Docs: %s\n", appConfig.Output.GoogleDocs.URL)
		fmt.Printf("📊 Summary: %d objectives with their key results and weekly updates\n", len(objectives))
		return nil
	} else if outputFormat == ports.OutputFormatGoogleDocs && appConfig.Output.GoogleDocs.URL != "" && (googleClientID == "" || googleClientSecret == "") {
		fmt.Printf("⚠️ Google Docs integration requested but missing credentials. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables.\n")
		fmt.Printf("📝 Falling back to plain text file generation...\n")
	}

	// Generate report to file
	if analysisResult != nil && analysisResult.Enabled && outputFormat == ports.OutputFormatMarkdown {
		// Use markdown with analysis for markdown format
		writer := output.NewWriterWithConfig(appConfig)
		err = writer.WriteMarkdownWithAnalysis(objectives, projectInfo, outputFile, analysisResult.Analysis)
	} else {
		// Use regular report generation
		err = reportGenerator.GenerateReport(objectives, projectInfo, outputFormat, outputFile)
	}
	if err != nil {
		return fmt.Errorf("error generating report: %v", err)
	}

	// Success message
	fmt.Printf("✅ Report generated successfully: %s\n", outputFile)

	// Calculate file size
	if fileInfo, err := os.Stat(outputFile); err == nil {
		fmt.Printf("📄 File size: %d bytes\n", fileInfo.Size())
	}

	// Summary message
	if outputFormat != ports.OutputFormatJSON {
		fmt.Printf("📊 Summary: %d objectives with their key results and weekly updates\n", len(objectives))
		if outputFormat == ports.OutputFormatGoogleDocs {
			fmt.Printf("📋 Google Docs compatible format - copy and paste the content directly into Google Docs\n")
		} else {
			fmt.Printf("🔗 Open the file to view the formatted OKR report with status indicators\n")
		}
	}

	return nil
}