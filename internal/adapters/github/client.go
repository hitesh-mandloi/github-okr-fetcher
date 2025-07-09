package github

import (
	"github.com/google/go-github/v58/github"
	
	"github-okr-fetcher/internal/domain/entity"
)

// GitHubClient wraps the bridge client
type GitHubClient struct {
	bridge *BridgeClient
}

// NewGitHubClient creates a new GitHub client
func NewGitHubClient(token string, config *entity.Config) *GitHubClient {
	bridge := NewBridgeClient(token, config)
	return &GitHubClient{
		bridge: bridge,
	}
}

// Bridge methods that delegate to the bridge implementation

func (c *GitHubClient) parseProjectURL(url string) (*entity.ProjectInfo, error) {
	return c.bridge.parseProjectURL(url)
}

func (c *GitHubClient) fetchProjectIssuesRobust(projectInfo *entity.ProjectInfo) ([]*github.Issue, error) {
	return c.bridge.fetchProjectIssuesRobust(projectInfo)
}

func (c *GitHubClient) fetchIssuesBySearchQuery(owner, repo, query string) ([]*github.Issue, error) {
	return c.bridge.fetchIssuesBySearchQuery(owner, repo, query)
}

func (c *GitHubClient) fetchIssueComments(owner, repo string, issueNumber int) ([]*github.IssueComment, error) {
	return c.bridge.fetchIssueComments(owner, repo, issueNumber)
}

func (c *GitHubClient) findParentIssueFromRelationships(owner, repo string, issueNumber int) (int, error) {
	return c.bridge.findParentIssueFromRelationships(owner, repo, issueNumber)
}

func (c *GitHubClient) testBasicAccess(org string) error {
	return c.bridge.testBasicAccess(org)
}

func (c *GitHubClient) listOrganizationProjects(org string) error {
	return c.bridge.listOrganizationProjects(org)
}