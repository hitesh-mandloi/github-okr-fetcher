package ports

import (
	"context"

	"github-okr-fetcher/internal/domain/entity"
)

// GitHubRepository defines the interface for GitHub data access
type GitHubRepository interface {
	// Project operations
	ParseProjectURL(url string) (*entity.ProjectInfo, error)
	FetchProjectIssues(ctx context.Context, projectInfo *entity.ProjectInfo) ([]*entity.Issue, error)
	
	// Issue operations
	FetchIssuesBySearch(ctx context.Context, owner, repo, query string) ([]*entity.Issue, error)
	FetchIssueComments(ctx context.Context, owner, repo string, issueNumber int) ([]*entity.WeeklyUpdate, error)
	
	// Relationship operations
	FindParentIssue(ctx context.Context, owner, repo string, issueNumber int) (int, error)
	
	// Utility operations
	ExtractOwnerRepoFromIssue(issue *entity.Issue) (owner, repo string)
	TestBasicAccess(ctx context.Context, org string) error
	ListOrganizationProjects(ctx context.Context, org string) error
}

// GitHubService defines high-level GitHub operations
type GitHubService interface {
	ProcessIssues(ctx context.Context, issues []*entity.Issue, requiredLabels []string) ([]*entity.IssueWithUpdates, error)
	FetchProjectIssuesRobust(ctx context.Context, projectInfo *entity.ProjectInfo) ([]*entity.Issue, error)
}