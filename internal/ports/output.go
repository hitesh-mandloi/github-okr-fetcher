package ports

import "github-okr-fetcher/internal/domain/entity"

// OutputFormat represents different output formats
type OutputFormat string

const (
	OutputFormatMarkdown OutputFormat = "markdown"
	OutputFormatJSON     OutputFormat = "json"
	OutputFormatGoogleDocs OutputFormat = "google-docs"
)

// OutputWriter defines the interface for writing output
type OutputWriter interface {
	WriteMarkdown(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, filename string) error
	WriteJSON(objectives []*entity.IssueWithUpdates, filename string) error
	WriteGoogleDocs(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, documentURL, clientID, clientSecret string) error
}

// ReportGenerator defines high-level report generation operations
type ReportGenerator interface {
	GenerateReport(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, format OutputFormat, filename string) error
	GenerateReportWithGoogleDocs(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, format OutputFormat, filename, documentURL, clientID, clientSecret string) error
	FormatAsMarkdown(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo) string
	FormatAsJSON(objectives []*entity.IssueWithUpdates) (string, error)
	FormatAsGoogleDocs(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo) string
}