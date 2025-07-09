package entity

// ProjectType represents the type of GitHub project
type ProjectType string

const (
	ProjectTypeOrganization ProjectType = "org"
	ProjectTypeRepository   ProjectType = "repo"
)

// ProjectInfo contains information about a GitHub project
type ProjectInfo struct {
	Owner     string      `json:"owner"`
	Repo      string      `json:"repo,omitempty"`
	ProjectID int         `json:"project_id"`
	ViewID    int         `json:"view_id,omitempty"`
	Type      ProjectType `json:"type"`
	URL       string      `json:"url,omitempty"`
}

// IsOrganizationProject returns true if this is an organization project
func (p *ProjectInfo) IsOrganizationProject() bool {
	return p.Type == ProjectTypeOrganization
}

// IsRepositoryProject returns true if this is a repository project
func (p *ProjectInfo) IsRepositoryProject() bool {
	return p.Type == ProjectTypeRepository
}

// HasView returns true if the project has a specific view
func (p *ProjectInfo) HasView() bool {
	return p.ViewID > 0
}

// Project represents a complete project with objectives and metadata
type Project struct {
	Info       *ProjectInfo           `json:"info"`
	Objectives []*IssueWithUpdates    `json:"objectives"`
}