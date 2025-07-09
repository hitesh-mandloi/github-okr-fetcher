package github

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/google/go-github/v58/github"

	"github-okr-fetcher/internal/domain/entity"
)

// Repository implements the GitHubRepository interface
type Repository struct {
	client *BridgeClient
}

// NewRepository creates a new GitHub repository adapter
func NewRepository(token string, config *entity.Config) *Repository {
	client := NewBridgeClient(token, config)
	return &Repository{
		client: client,
	}
}

// ParseProjectURL parses a GitHub project URL and returns project information
func (r *Repository) ParseProjectURL(url string) (*entity.ProjectInfo, error) {
	return r.client.parseProjectURL(url)
}

// FetchProjectIssues fetches issues from a GitHub project
func (r *Repository) FetchProjectIssues(ctx context.Context, projectInfo *entity.ProjectInfo) ([]*entity.Issue, error) {
	githubIssues, err := r.client.fetchProjectIssuesRobust(projectInfo)
	if err != nil {
		return nil, err
	}

	return r.convertGitHubIssuesToDomain(githubIssues), nil
}

// FetchIssuesBySearch searches for issues using GitHub's search API
func (r *Repository) FetchIssuesBySearch(ctx context.Context, owner, repo, query string) ([]*entity.Issue, error) {
	githubIssues, err := r.client.fetchIssuesBySearchQuery(owner, repo, query)
	if err != nil {
		return nil, err
	}

	return r.convertGitHubIssuesToDomain(githubIssues), nil
}

// FetchIssueComments fetches comments from a GitHub issue and extracts weekly updates
func (r *Repository) FetchIssueComments(ctx context.Context, owner, repo string, issueNumber int) ([]*entity.WeeklyUpdate, error) {
	comments, err := r.client.fetchIssueComments(owner, repo, issueNumber)
	if err != nil {
		return nil, err
	}

	return r.convertGitHubCommentsToWeeklyUpdates(comments), nil
}

// FindParentIssue attempts to find the parent issue of a given issue
func (r *Repository) FindParentIssue(ctx context.Context, owner, repo string, issueNumber int) (int, error) {
	return r.client.findParentIssueFromRelationships(owner, repo, issueNumber)
}

// ExtractOwnerRepoFromIssue extracts owner and repo from an issue URL
func (r *Repository) ExtractOwnerRepoFromIssue(issue *entity.Issue) (owner, repo string) {
	if issue.URL == "" {
		return "", ""
	}

	re := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+)/issues/\d+`)
	matches := re.FindStringSubmatch(issue.URL)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	return "", ""
}

// TestBasicAccess tests basic access to GitHub organization
func (r *Repository) TestBasicAccess(ctx context.Context, org string) error {
	return r.client.testBasicAccess(org)
}

// ListOrganizationProjects lists projects in a GitHub organization
func (r *Repository) ListOrganizationProjects(ctx context.Context, org string) error {
	return r.client.listOrganizationProjects(org)
}

// Helper methods

func (r *Repository) convertGitHubIssuesToDomain(githubIssues []*github.Issue) []*entity.Issue {
	var issues []*entity.Issue

	for _, ghIssue := range githubIssues {
		if ghIssue.Number == nil || ghIssue.Title == nil || ghIssue.HTMLURL == nil {
			continue
		}

		var labels []string
		for _, label := range ghIssue.Labels {
			if label.Name != nil {
				labels = append(labels, *label.Name)
			}
		}

		var body, state string
		if ghIssue.Body != nil {
			body = *ghIssue.Body
		}
		if ghIssue.State != nil {
			state = *ghIssue.State
		}

		issue := &entity.Issue{
			Number: *ghIssue.Number,
			Title:  *ghIssue.Title,
			URL:    *ghIssue.HTMLURL,
			Body:   body,
			State:  state,
			Labels: labels,
		}

		issues = append(issues, issue)
	}

	return issues
}

// convertGitHubCommentsToWeeklyUpdates converts GitHub comments to weekly updates
func (r *Repository) convertGitHubCommentsToWeeklyUpdates(comments []*github.IssueComment) []*entity.WeeklyUpdate {
	var updates []*entity.WeeklyUpdate

	for _, comment := range comments {
		if comment.Body == nil || comment.User == nil || comment.User.Login == nil || comment.CreatedAt == nil {
			continue
		}

		body := *comment.Body

		// Look for weekly update pattern
		weeklyUpdatePattern := regexp.MustCompile(`(?i)weekly\s+update\s+(\d{4}-\d{2}-\d{2})`)
		if !weeklyUpdatePattern.MatchString(body) {
			continue
		}

		matches := weeklyUpdatePattern.FindStringSubmatch(body)
		var date string
		if len(matches) > 1 {
			date = matches[1]
		} else {
			date = comment.CreatedAt.Format("2006-01-02")
		}

		// Detect status from content
		status := r.detectStatusFromContent(body)

		update := &entity.WeeklyUpdate{
			Date:    date,
			Content: body,
			Author:  *comment.User.Login,
			Status:  status,
		}

		updates = append(updates, update)
	}

	// Sort by date descending (most recent first)
	sort.Slice(updates, func(i, j int) bool {
		return updates[i].Date > updates[j].Date
	})

	return updates
}

// detectStatusFromContent detects status from comment content based on colors, emojis, and text
func (r *Repository) detectStatusFromContent(content string) entity.WeeklyUpdateStatus {
	contentLower := strings.ToLower(content)

	// Check for completion indicators first (highest priority)
	if strings.Contains(contentLower, "completed") || strings.Contains(contentLower, "done") || strings.Contains(contentLower, "finished") {
		return entity.StatusCompleted
	}

	// Check for blocked indicators (red color/emoji)
	if strings.Contains(content, "🔴") || strings.Contains(content, "🚫") || 
		strings.Contains(contentLower, "red") || strings.Contains(contentLower, "blocked") || 
		strings.Contains(contentLower, "stuck") || strings.Contains(contentLower, "cannot") {
		return entity.StatusBlocked
	}

	// Check for delayed indicators (red color/emoji) 
	if strings.Contains(content, "🔴") && (strings.Contains(contentLower, "delay") || strings.Contains(contentLower, "behind")) ||
		strings.Contains(contentLower, "delayed") {
		return entity.StatusDelayed
	}

	// Check for caution indicators (yellow color/emoji)
	if strings.Contains(content, "🟡") || strings.Contains(content, "⚠️") || 
		strings.Contains(contentLower, "yellow") || strings.Contains(contentLower, "caution") ||
		strings.Contains(contentLower, "warning") {
		return entity.StatusCaution
	}

	// Check for at-risk indicators
	if strings.Contains(contentLower, "at risk") || strings.Contains(contentLower, "at-risk") ||
		strings.Contains(contentLower, "risk") {
		return entity.StatusAtRisk
	}

	// Check for on-track indicators (green color/emoji)
	if strings.Contains(content, "🟢") || strings.Contains(content, "✅") ||
		strings.Contains(contentLower, "green") || strings.Contains(contentLower, "on track") || 
		strings.Contains(contentLower, "on-track") || strings.Contains(contentLower, "progress") {
		return entity.StatusOnTrack
	}

	return entity.StatusUnknown
}
