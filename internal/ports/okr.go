package ports

import (
	"context"

	"github-okr-fetcher/internal/domain/entity"
)

// OKRService defines the main business logic interface for OKR operations
type OKRService interface {
	// Main OKR operations
	FetchOKRData(ctx context.Context, config *entity.Config) ([]*entity.IssueWithUpdates, *entity.ProjectInfo, error)
	ProcessOKRIssues(ctx context.Context, issues []*entity.Issue, requiredLabels []string) ([]*entity.IssueWithUpdates, error)
	
	// Issue relationship operations
	BuildParentChildRelationships(ctx context.Context, issues []*entity.Issue) (map[int][]*entity.Issue, error)
	IdentifyObjectivesAndKeyResults(issues []*entity.Issue, parentChildMap map[int][]*entity.Issue) ([]*entity.Issue, error)
	
	// Weekly update operations
	ExtractWeeklyUpdates(updates []string) []*entity.WeeklyUpdate
	DetectStatusFromContent(content string) entity.WeeklyUpdateStatus
}