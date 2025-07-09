package service

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github-okr-fetcher/internal/domain/entity"
	"github-okr-fetcher/internal/ports"
)

// OKRService implements the main business logic for OKR operations
type OKRService struct {
	githubRepo ports.GitHubRepository
}

// NewOKRService creates a new OKR service
func NewOKRService(githubRepo ports.GitHubRepository) *OKRService {
	return &OKRService{
		githubRepo: githubRepo,
	}
}

// FetchOKRData retrieves and processes OKR data from GitHub
func (s *OKRService) FetchOKRData(ctx context.Context, config *entity.Config) ([]*entity.IssueWithUpdates, *entity.ProjectInfo, error) {
	// Parse project URL
	projectInfo, err := s.githubRepo.ParseProjectURL(config.GitHub.ProjectURL)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing project URL: %w", err)
	}

	// Fetch issues
	var issues []*entity.Issue
	if config.ShouldUseSearch() {
		searchQuery := config.GetSearchQuery()
		owner := config.GitHub.Owner
		repo := config.GitHub.Repo
		if owner == "" {
			owner = projectInfo.Owner
		}
		if repo == "" {
			repo = "microservices" // Default
		}

		issues, err = s.githubRepo.FetchIssuesBySearch(ctx, owner, repo, searchQuery)
		if err != nil || len(issues) == 0 {
			// Fallback to project-based query
			issues, err = s.githubRepo.FetchProjectIssues(ctx, projectInfo)
			if err != nil {
				return nil, nil, fmt.Errorf("error fetching issues: %w", err)
			}
		}
	} else {
		issues, err = s.githubRepo.FetchProjectIssues(ctx, projectInfo)
		if err != nil {
			return nil, nil, fmt.Errorf("error fetching project issues: %w", err)
		}
	}

	// Process issues
	objectives, err := s.ProcessOKRIssues(ctx, issues, config.GetLabels())
	if err != nil {
		return nil, nil, fmt.Errorf("error processing issues: %w", err)
	}

	return objectives, projectInfo, nil
}

// ProcessOKRIssues processes a list of issues and organizes them into objectives and key results
func (s *OKRService) ProcessOKRIssues(ctx context.Context, issues []*entity.Issue, requiredLabels []string) ([]*entity.IssueWithUpdates, error) {
	log.Printf("🔄 Processing %d issues with %d required labels", len(issues), len(requiredLabels))

	// Filter issues by required labels
	filteredIssues := s.filterIssuesByLabels(issues, requiredLabels)
	log.Printf("📋 %d issues passed label filtering", len(filteredIssues))

	// If no issues after filtering, return empty
	if len(filteredIssues) == 0 {
		return []*entity.IssueWithUpdates{}, nil
	}

	// Build parent-child relationships
	parentChildMap, err := s.BuildParentChildRelationships(ctx, filteredIssues)
	if err != nil {
		log.Printf("⚠️  Error building parent-child relationships: %v", err)
		// Continue without relationships
		parentChildMap = make(map[int][]*entity.Issue)
	}

	// Identify objectives (issues without parents) and key results (issues with parents)
	parentIssues, err := s.IdentifyObjectivesAndKeyResults(filteredIssues, parentChildMap)
	if err != nil {
		log.Printf("⚠️  Error identifying objectives: %v", err)
	}

	// If no parent issues found, treat all issues as objectives (fallback behavior)
	if len(parentIssues) == 0 {
		log.Printf("⚠️  No parent-child relationships found, treating all issues as objectives")
		for _, issue := range filteredIssues {
			issue.Type = entity.IssueTypeObjective
			parentIssues = append(parentIssues, issue)
		}
	}

	// Process each objective with its key results
	var objectives []*entity.IssueWithUpdates
	for _, objective := range parentIssues {
		children := parentChildMap[objective.Number]
		objectiveWithUpdates, err := s.processObjectiveWithChildren(ctx, objective, children)
		if err != nil {
			log.Printf("⚠️  Error processing objective #%d: %v", objective.Number, err)
			continue
		}
		objectives = append(objectives, objectiveWithUpdates)
	}

	log.Printf("✅ Processed into %d objectives with %d total key results",
		len(objectives), s.countTotalKeyResults(objectives))
	return objectives, nil
}

// countTotalKeyResults counts the total number of key results across all objectives
func (s *OKRService) countTotalKeyResults(objectives []*entity.IssueWithUpdates) int {
	total := 0
	for _, obj := range objectives {
		total += len(obj.ChildIssues)
	}
	return total
}

// processIssueWithUpdates processes a single issue and fetches its updates
func (s *OKRService) processIssueWithUpdates(ctx context.Context, issue *entity.Issue) (*entity.IssueWithUpdates, error) {
	// Extract owner and repo from issue URL
	owner, repo := s.githubRepo.ExtractOwnerRepoFromIssue(issue)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("could not extract owner/repo from issue #%d", issue.Number)
	}

	// Fetch updates for this issue
	updates, err := s.githubRepo.FetchIssueComments(ctx, owner, repo, issue.Number)
	if err != nil {
		log.Printf("⚠️  Error fetching updates for issue #%d: %v", issue.Number, err)
		updates = []*entity.WeeklyUpdate{} // Continue with empty updates
	}

	// Convert to the format expected by IssueWithUpdates
	var allUpdates []entity.WeeklyUpdate
	for _, update := range updates {
		allUpdates = append(allUpdates, *update)
	}

	// Set latest update
	var latestUpdate *entity.WeeklyUpdate
	if len(updates) > 0 {
		latestUpdate = updates[0] // Updates are already sorted by date descending
	}

	return &entity.IssueWithUpdates{
		Issue:        *issue,
		LatestUpdate: latestUpdate,
		AllUpdates:   allUpdates,
		ChildIssues:  []entity.IssueWithUpdates{}, // No child issues for now
	}, nil
}

// BuildParentChildRelationships analyzes issues to build parent-child relationships
func (s *OKRService) BuildParentChildRelationships(ctx context.Context, issues []*entity.Issue) (map[int][]*entity.Issue, error) {
	parentChildMap := make(map[int][]*entity.Issue)

	for _, issue := range issues {
		parentNum := s.extractParentIssueNumber(issue)

		// If no parent found in body, try to find relationships via GitHub API
		if parentNum == 0 {
			owner, repo := s.githubRepo.ExtractOwnerRepoFromIssue(issue)
			if owner != "" && repo != "" {
				apiParentNum, err := s.githubRepo.FindParentIssue(ctx, owner, repo, issue.Number)
				if err == nil && apiParentNum > 0 {
					parentNum = apiParentNum
				}
			}
		}

		if parentNum > 0 {
			parentChildMap[parentNum] = append(parentChildMap[parentNum], issue)
		}
	}

	return parentChildMap, nil
}

// IdentifyObjectivesAndKeyResults identifies which issues are objectives vs key results
func (s *OKRService) IdentifyObjectivesAndKeyResults(issues []*entity.Issue, parentChildMap map[int][]*entity.Issue) ([]*entity.Issue, error) {
	var parentIssues []*entity.Issue

	for _, issue := range issues {
		// Check if this issue has children but no parent
		hasChildren := len(parentChildMap[issue.Number]) > 0
		hasParent := s.hasParentIssue(issue, parentChildMap)

		if hasChildren && !hasParent {
			issue.Type = entity.IssueTypeObjective
			parentIssues = append(parentIssues, issue)
		} else if hasParent {
			issue.Type = entity.IssueTypeKeyResult
		}
	}

	return parentIssues, nil
}

// ExtractWeeklyUpdates extracts weekly updates from comment strings
func (s *OKRService) ExtractWeeklyUpdates(comments []string) []*entity.WeeklyUpdate {
	var updates []*entity.WeeklyUpdate
	weeklyUpdateRegex := regexp.MustCompile(`(?mi)^#\s*weekly\s+update\s+(\d{4}-\d{2}-\d{2})`)

	for _, comment := range comments {
		matches := weeklyUpdateRegex.FindStringSubmatch(comment)
		if len(matches) >= 2 {
			status := s.DetectStatusFromContent(comment)
			update := &entity.WeeklyUpdate{
				Date:    matches[1],
				Content: comment,
				Author:  "unknown", // Would need to be passed in from comment metadata
				Status:  status,
			}
			updates = append(updates, update)
		}
	}

	// Sort by date (newest first)
	sort.Slice(updates, func(i, j int) bool {
		return updates[i].Date > updates[j].Date
	})

	return updates
}

// DetectStatusFromContent analyzes content to determine status
func (s *OKRService) DetectStatusFromContent(content string) entity.WeeklyUpdateStatus {
	contentLower := strings.ToLower(content)

	// Check for completion indicators
	if strings.Contains(contentLower, "completed") ||
		strings.Contains(contentLower, "done") ||
		strings.Contains(contentLower, "finished") ||
		strings.Contains(contentLower, "✅") ||
		strings.Contains(contentLower, "✓") {
		return entity.StatusCompleted
	}

	// Check for blocked indicators
	if strings.Contains(contentLower, "blocked") ||
		strings.Contains(contentLower, "stuck") ||
		strings.Contains(contentLower, "issue") ||
		strings.Contains(contentLower, "problem") ||
		strings.Contains(contentLower, "🚫") ||
		strings.Contains(contentLower, "❌") {
		return entity.StatusBlocked
	}

	// Check for at-risk indicators
	if strings.Contains(contentLower, "behind") ||
		strings.Contains(contentLower, "delayed") ||
		strings.Contains(contentLower, "risk") ||
		strings.Contains(contentLower, "concern") ||
		strings.Contains(contentLower, "⚠️") ||
		strings.Contains(contentLower, "🟡") {
		return entity.StatusAtRisk
	}

	// Check for on-track indicators
	if strings.Contains(contentLower, "on track") ||
		strings.Contains(contentLower, "progress") ||
		strings.Contains(contentLower, "good") ||
		strings.Contains(contentLower, "🟢") ||
		strings.Contains(contentLower, "✅") {
		return entity.StatusOnTrack
	}

	// Default to on-track if no specific indicators found
	return entity.StatusOnTrack
}

// Helper methods

func (s *OKRService) filterIssuesByLabels(issues []*entity.Issue, requiredLabels []string) []*entity.Issue {
	if len(requiredLabels) == 0 {
		return issues
	}

	var filtered []*entity.Issue
	for _, issue := range issues {
		if issue.HasAllLabels(requiredLabels) {
			filtered = append(filtered, issue)
		}
	}

	return filtered
}

func (s *OKRService) extractParentIssueNumber(issue *entity.Issue) int {
	// Check both title and body for parent references
	textToSearch := issue.Title + "\n" + issue.Body

	// Patterns to look for parent issue references
	bodyPatterns := []string{
		`(?i)parent\s*(?:issue)?\s*:?\s*#(\d+)`,
		`(?i)parent\s*(?:issue)?\s*:?\s*https://github\.com/[^/]+/[^/]+/issues/(\d+)`,
		`(?i)part\s*of\s*#(\d+)`,
		`(?i)child\s*of\s*#(\d+)`,
		`(?i)subtask\s*of\s*#(\d+)`,
		`(?i)depends\s*on\s*#(\d+)`,
		`(?i)relates\s*to\s*#(\d+)`,
		`(?i)blocking\s*#(\d+)`,
		`(?i)blocked\s*by\s*#(\d+)`,
	}

	for _, pattern := range bodyPatterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindStringSubmatch(textToSearch)
		if len(matches) >= 2 {
			if parentNum, err := strconv.Atoi(matches[1]); err == nil {
				log.Printf("📎 Found parent reference in issue #%d: parent is #%d", issue.Number, parentNum)
				return parentNum
			}
		}
	}

	return 0
}

func (s *OKRService) hasParentIssue(issue *entity.Issue, parentChildMap map[int][]*entity.Issue) bool {
	parentNum := s.extractParentIssueNumber(issue)
	return parentNum > 0
}

func (s *OKRService) processObjectiveWithChildren(ctx context.Context, objective *entity.Issue, children []*entity.Issue) (*entity.IssueWithUpdates, error) {
	// Fetch updates for objective
	owner, repo := s.githubRepo.ExtractOwnerRepoFromIssue(objective)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("could not extract owner/repo from issue #%d", objective.Number)
	}

	updates, err := s.githubRepo.FetchIssueComments(ctx, owner, repo, objective.Number)
	if err != nil {
		log.Printf("Warning: Could not fetch comments for issue #%d: %v", objective.Number, err)
	}

	var latestUpdate *entity.WeeklyUpdate
	if len(updates) > 0 {
		latestUpdate = updates[0]
	}

	// Convert slice to match entity structure
	var allUpdates []entity.WeeklyUpdate
	for _, update := range updates {
		allUpdates = append(allUpdates, *update)
	}

	objectiveWithUpdates := &entity.IssueWithUpdates{
		Issue:        *objective,
		LatestUpdate: latestUpdate,
		AllUpdates:   allUpdates,
	}

	// Process children (key results)
	for _, child := range children {
		child.Type = entity.IssueTypeKeyResult

		childOwner, childRepo := s.githubRepo.ExtractOwnerRepoFromIssue(child)
		if childOwner == "" || childRepo == "" {
			log.Printf("Warning: Could not extract owner/repo from child issue #%d", child.Number)
			continue
		}

		childUpdates, err := s.githubRepo.FetchIssueComments(ctx, childOwner, childRepo, child.Number)
		if err != nil {
			log.Printf("Warning: Could not fetch comments for issue #%d: %v", child.Number, err)
		}

		var childLatestUpdate *entity.WeeklyUpdate
		if len(childUpdates) > 0 {
			childLatestUpdate = childUpdates[0]
		}

		// Convert child updates slice
		var childAllUpdates []entity.WeeklyUpdate
		for _, update := range childUpdates {
			childAllUpdates = append(childAllUpdates, *update)
		}

		childWithUpdates := entity.IssueWithUpdates{
			Issue:        *child,
			LatestUpdate: childLatestUpdate,
			AllUpdates:   childAllUpdates,
		}

		objectiveWithUpdates.ChildIssues = append(objectiveWithUpdates.ChildIssues, childWithUpdates)
	}

	return objectiveWithUpdates, nil
}
