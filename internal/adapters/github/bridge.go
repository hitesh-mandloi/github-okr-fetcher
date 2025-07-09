package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v58/github"
	"golang.org/x/oauth2"

	"github-okr-fetcher/internal/domain/entity"
)

// This file provides a bridge to the existing GitHub client implementation
// until we can fully migrate all functionality

// BridgeClient provides access to the GitHub API with enhanced functionality
type BridgeClient struct {
	client      *github.Client
	httpClient  *http.Client
	ctx         context.Context
	token       string
	rateLimiter *RateLimiter
	cache       *APICache
	stats       *ClientStats
	config      *entity.Config
	mu          sync.RWMutex
}

// NewBridgeClient creates a new bridge client with enhanced functionality
func NewBridgeClient(token string, config *entity.Config) *BridgeClient {
	ctx := context.Background()

	// Get timeout from config or use default
	timeoutSec := 30
	if config != nil && config.GitHub.TimeoutSec > 0 {
		timeoutSec = config.GitHub.TimeoutSec
	}
	timeout := time.Duration(timeoutSec) * time.Second

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: timeout,
	}

	// Create OAuth2 client
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	tc.Timeout = timeout

	client := github.NewClient(tc)

	// Configure rate limiter
	rateLimit := 5000 // Default GitHub rate limit
	if config != nil && config.GitHub.RateLimit > 0 {
		rateLimit = config.GitHub.RateLimit
	} else if config != nil && config.Performance.RateLimit > 0 {
		rateLimit = config.Performance.RateLimit
	}
	rateLimiter := NewRateLimiter(rateLimit)

	// Initialize cache if enabled
	var cache *APICache
	if config != nil && config.Performance.CacheEnabled {
		cache = NewAPICache()
	}

	return &BridgeClient{
		client:      client,
		httpClient:  httpClient,
		ctx:         ctx,
		token:       token,
		rateLimiter: rateLimiter,
		cache:       cache,
		stats:       &ClientStats{},
		config:      config,
	}
}

// GetStats returns a copy of the current client statistics
func (b *BridgeClient) GetStats() ClientStats {
	return b.stats.GetStats()
}

// parseProjectURL parses a GitHub project URL
func (b *BridgeClient) parseProjectURL(url string) (*entity.ProjectInfo, error) {
	patterns := []struct {
		regex string
		isOrg bool
	}{
		{`https://github\.com/orgs/([^/]+)/projects/(\d+)/views/(\d+)`, true},
		{`https://github\.com/orgs/([^/]+)/projects/(\d+)`, true},
		{`https://github\.com/([^/]+)/([^/]+)/projects/(\d+)/views/(\d+)`, false},
		{`https://github\.com/([^/]+)/([^/]+)/projects/(\d+)`, false},
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern.regex)
		matches := re.FindStringSubmatch(url)

		if len(matches) >= 3 {
			var projectID int
			var err error

			if pattern.isOrg {
				projectID, err = strconv.Atoi(matches[2])
				if err != nil {
					return nil, fmt.Errorf("invalid project ID: %v", err)
				}

				info := &entity.ProjectInfo{
					Owner:     matches[1],
					ProjectID: projectID,
					Type:      entity.ProjectTypeOrganization,
					URL:       url,
				}

				// Check for view ID
				if len(matches) >= 4 {
					viewID, err := strconv.Atoi(matches[3])
					if err != nil {
						return nil, fmt.Errorf("invalid view ID: %v", err)
					}
					info.ViewID = viewID
				}

				return info, nil
			} else {
				projectID, err = strconv.Atoi(matches[3])
				if err != nil {
					return nil, fmt.Errorf("invalid project ID: %v", err)
				}

				info := &entity.ProjectInfo{
					Owner:     matches[1],
					Repo:      matches[2],
					ProjectID: projectID,
					Type:      entity.ProjectTypeRepository,
					URL:       url,
				}

				// Check for view ID
				if len(matches) >= 5 {
					viewID, err := strconv.Atoi(matches[4])
					if err != nil {
						return nil, fmt.Errorf("invalid view ID: %v", err)
					}
					info.ViewID = viewID
				}

				return info, nil
			}
		}
	}

	return nil, fmt.Errorf("invalid GitHub project URL format")
}

// waitForRateLimit waits for rate limit if necessary
func (b *BridgeClient) waitForRateLimit() error {
	return b.rateLimiter.Wait(b.ctx)
}

// retryWithBackoff retries an operation with exponential backoff
func (b *BridgeClient) retryWithBackoff(maxRetries int, operation func() error) error {
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			b.stats.IncrementRetry()
			// Exponential backoff
			delay := time.Duration(i*i) * time.Second
			log.Printf("🔄 Retrying operation after %v (attempt %d/%d)", delay, i+1, maxRetries)
			time.Sleep(delay)
		}

		if err := operation(); err != nil {
			if i == maxRetries-1 || !b.isRetryableError(err) {
				b.stats.IncrementError()
				return err
			}
			log.Printf("⚠️  Operation failed, will retry: %v", err)
			continue
		}

		return nil
	}

	return fmt.Errorf("operation failed after %d retries", maxRetries)
}

// isRetryableError checks if an error is retryable
func (b *BridgeClient) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := strings.ToLower(err.Error())
	retryableErrors := []string{
		"timeout",
		"connection reset",
		"connection refused",
		"temporary failure",
		"rate limit",
		"server error",
		"503",
		"502",
		"500",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errorStr, retryable) {
			return true
		}
	}

	return false
}

// updateRateLimitStats updates rate limit statistics from HTTP response headers
func (b *BridgeClient) updateRateLimitStats(resp *http.Response) {
	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			var resetTime time.Time
			if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
				if resetVal, err := strconv.ParseInt(reset, 10, 64); err == nil {
					resetTime = time.Unix(resetVal, 0)
				}
			}
			b.stats.UpdateQuota(val, resetTime)
		}
	}

	if resp.StatusCode == 429 {
		b.stats.IncrementRateLimitHit()
	}
}

// fetchProjectIssuesRobust fetches issues from a GitHub project with robust error handling
func (b *BridgeClient) fetchProjectIssuesRobust(projectInfo *entity.ProjectInfo) ([]*github.Issue, error) {
	log.Printf("🎯 Fetching issues from project %d (owner: %s, type: %s)",
		projectInfo.ProjectID, projectInfo.Owner, projectInfo.Type)

	// For now, fallback to search-based approach
	query := "is:issue"
	owner := projectInfo.Owner
	repo := "microservices" // Default repo

	if b.config != nil {
		if b.config.GitHub.Owner != "" {
			owner = b.config.GitHub.Owner
		}
		if b.config.GitHub.Repo != "" {
			repo = b.config.GitHub.Repo
		}
	}

	return b.fetchIssuesBySearchQuery(owner, repo, query)
}

// fetchIssuesBySearchQuery fetches issues using GitHub search API with pagination
func (b *BridgeClient) fetchIssuesBySearchQuery(owner, repo, searchQuery string) ([]*github.Issue, error) {
	if searchQuery == "" {
		return nil, fmt.Errorf("no search query specified")
	}

	log.Printf("🔍 Searching repository %s/%s with query: %s", owner, repo, searchQuery)

	// Check cache first
	cacheKey := fmt.Sprintf("search:%s/%s:%s", owner, repo, searchQuery)
	if b.cache != nil {
		if cached, found := b.cache.GetFromCache(cacheKey); found {
			if issues, ok := cached.([]*github.Issue); ok {
				b.stats.IncrementCacheHit()
				log.Printf("📊 Found %d issues from cache", len(issues))
				return issues, nil
			}
		}
	}

	var allIssues []*github.Issue

	operation := func() error {
		pageSize := 100
		if b.config != nil && b.config.GitHub.PageSize > 0 {
			pageSize = b.config.GitHub.PageSize
		}
		
		opt := &github.SearchOptions{
			ListOptions: github.ListOptions{
				PerPage: pageSize,
			},
		}

		for {
			// Wait for rate limit
			if err := b.waitForRateLimit(); err != nil {
				return fmt.Errorf("rate limit error: %v", err)
			}

			b.stats.IncrementAPICall()
			result, resp, err := b.client.Search.Issues(b.ctx, fmt.Sprintf("repo:%s/%s %s", owner, repo, searchQuery), opt)
			if err != nil {
				return fmt.Errorf("error searching issues: %v", err)
			}

			b.updateRateLimitStats(resp.Response)
			allIssues = append(allIssues, result.Issues...)

			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage

			// Prevent unlimited memory growth
			maxIssues := 10000
			if b.config != nil && b.config.GitHub.MaxIssuesLimit > 0 {
				maxIssues = b.config.GitHub.MaxIssuesLimit
			}
			if len(allIssues) > maxIssues {
				log.Printf("⚠️  Limiting results to %d issues to prevent memory issues", maxIssues)
				break
			}
		}

		return nil
	}

	maxRetries := 3
	if b.config != nil && b.config.GitHub.MaxRetries > 0 {
		maxRetries = b.config.GitHub.MaxRetries
	}
	if err := b.retryWithBackoff(maxRetries, operation); err != nil {
		return nil, err
	}

	// Cache successful response
	if b.cache != nil {
		cacheTTL := 10 * time.Minute
		if b.config != nil && b.config.Cache.IssuesTTLMin > 0 {
			cacheTTL = time.Duration(b.config.Cache.IssuesTTLMin) * time.Minute
		}
		b.cache.SetCache(cacheKey, allIssues, cacheTTL)
	}

	log.Printf("📊 Found %d issues matching search query", len(allIssues))
	return allIssues, nil
}

// fetchIssueComments fetches comments from a GitHub issue
func (b *BridgeClient) fetchIssueComments(owner, repo string, issueNumber int) ([]*github.IssueComment, error) {
	log.Printf("📝 Fetching comments for issue #%d in %s/%s", issueNumber, owner, repo)

	// Check cache first
	cacheKey := fmt.Sprintf("comments:%s/%s:%d", owner, repo, issueNumber)
	if b.cache != nil {
		if cached, found := b.cache.GetFromCache(cacheKey); found {
			if comments, ok := cached.([]*github.IssueComment); ok {
				b.stats.IncrementCacheHit()
				return comments, nil
			}
		}
	}

	var allComments []*github.IssueComment

	operation := func() error {
		opt := &github.IssueListCommentsOptions{
			ListOptions: github.ListOptions{
				PerPage: 100,
			},
		}

		for {
			// Wait for rate limit
			if err := b.waitForRateLimit(); err != nil {
				return fmt.Errorf("rate limit error: %v", err)
			}

			b.stats.IncrementAPICall()
			comments, resp, err := b.client.Issues.ListComments(b.ctx, owner, repo, issueNumber, opt)
			if err != nil {
				return fmt.Errorf("error fetching comments: %v", err)
			}

			b.updateRateLimitStats(resp.Response)
			allComments = append(allComments, comments...)

			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}

		return nil
	}

	if err := b.retryWithBackoff(3, operation); err != nil {
		return nil, err
	}

	// Cache the results
	if b.cache != nil {
		b.cache.SetCache(cacheKey, allComments, 5*time.Minute)
	}

	log.Printf("📊 Found %d comments for issue #%d", len(allComments), issueNumber)
	return allComments, nil
}

// findParentIssueFromRelationships attempts to find parent issue relationships
func (b *BridgeClient) findParentIssueFromRelationships(owner, repo string, issueNumber int) (int, error) {
	// This could be implemented to check GitHub issue relationships
	// For now, return 0 (no parent found)
	return 0, nil
}

// testBasicAccess tests basic access to GitHub organization
func (b *BridgeClient) testBasicAccess(org string) error {
	operation := func() error {
		if err := b.waitForRateLimit(); err != nil {
			return fmt.Errorf("rate limit error: %v", err)
		}

		b.stats.IncrementAPICall()
		_, resp, err := b.client.Organizations.Get(b.ctx, org)
		if err != nil {
			return fmt.Errorf("failed to access organization %s: %v", org, err)
		}

		if resp != nil {
			b.updateRateLimitStats(resp.Response)
		}

		return nil
	}

	return b.retryWithBackoff(3, operation)
}

// listOrganizationProjects lists projects in a GitHub organization
func (b *BridgeClient) listOrganizationProjects(org string) error {
	operation := func() error {
		if err := b.waitForRateLimit(); err != nil {
			return fmt.Errorf("rate limit error: %v", err)
		}

		b.stats.IncrementAPICall()
		projects, resp, err := b.client.Organizations.ListProjects(b.ctx, org, nil)
		if err != nil {
			return fmt.Errorf("failed to list projects for organization %s: %v", org, err)
		}

		if resp != nil {
			b.updateRateLimitStats(resp.Response)
		}

		log.Printf("📊 Found %d projects in organization %s", len(projects), org)
		for _, project := range projects {
			if project.Name != nil && project.ID != nil {
				log.Printf("  - %s (ID: %d)", *project.Name, *project.ID)
			}
		}

		return nil
	}

	return b.retryWithBackoff(3, operation)
}

// executeGraphQLQuery executes a GraphQL query against GitHub API
func (b *BridgeClient) executeGraphQLQuery(query string, variables map[string]interface{}) (*GraphQLResponse, error) {
	// Create cache key
	cacheKey := fmt.Sprintf("graphql:%x", Hash(query+fmt.Sprint(variables)))

	// Check cache first
	if b.cache != nil {
		if cached, found := b.cache.GetFromCache(cacheKey); found {
			if response, ok := cached.(*GraphQLResponse); ok {
				b.stats.IncrementCacheHit()
				return response, nil
			}
		}
	}

	var response *GraphQLResponse

	operation := func() error {
		requestBody := map[string]interface{}{
			"query":     query,
			"variables": variables,
		}

		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("error marshaling request: %v", err)
		}

		req, err := http.NewRequestWithContext(b.ctx, "POST", "https://api.github.com/graphql", bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("error creating request: %v", err)
		}

		req.Header.Set("Authorization", "Bearer "+b.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "GitHub-OKR-Fetcher/1.0")

		// Wait for rate limit
		if err := b.waitForRateLimit(); err != nil {
			return fmt.Errorf("rate limit error: %v", err)
		}

		b.stats.IncrementAPICall()
		resp, err := b.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("error executing request: %v", err)
		}
		defer resp.Body.Close()

		// Update rate limit stats
		b.updateRateLimitStats(resp)

		var graphqlResp GraphQLResponse
		if err := json.NewDecoder(resp.Body).Decode(&graphqlResp); err != nil {
			return fmt.Errorf("error decoding response: %v", err)
		}

		if len(graphqlResp.Errors) > 0 {
			return fmt.Errorf("GraphQL errors: %v", graphqlResp.Errors)
		}

		response = &graphqlResp
		return nil
	}

	if err := b.retryWithBackoff(3, operation); err != nil {
		return nil, err
	}

	// Cache successful response
	if b.cache != nil {
		b.cache.SetCache(cacheKey, response, 5*time.Minute)
	}

	return response, nil
}

// hasRequiredLabels checks if an issue has all required labels
func (b *BridgeClient) hasRequiredLabels(issue *github.Issue, requiredLabels []string) bool {
	if len(requiredLabels) == 0 {
		return true
	}

	issueLabels := make(map[string]bool)
	for _, label := range issue.Labels {
		if label.Name != nil {
			issueLabels[strings.ToLower(*label.Name)] = true
		}
	}

	for _, required := range requiredLabels {
		if !issueLabels[strings.ToLower(required)] {
			return false
		}
	}

	return true
}

// GraphQL response structures
type GraphQLResponse struct {
	Data struct {
		Organization struct {
			ProjectV2 struct {
				Items struct {
					PageInfo PageInfo   `json:"pageInfo"`
					Nodes    []ItemNode `json:"nodes"`
				} `json:"items"`
			} `json:"projectV2"`
		} `json:"organization"`
		Repository struct {
			ProjectV2 struct {
				Items struct {
					PageInfo PageInfo   `json:"pageInfo"`
					Nodes    []ItemNode `json:"nodes"`
				} `json:"items"`
			} `json:"projectV2"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// PageInfo contains pagination information
type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

// ItemNode represents a project item node from GraphQL
type ItemNode struct {
	Type    string `json:"type"`
	Content struct {
		Number     int    `json:"number"`
		Title      string `json:"title"`
		URL        string `json:"url"`
		State      string `json:"state"`
		Body       string `json:"body"`
		Repository struct {
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
			Name string `json:"name"`
		} `json:"repository"`
		Labels struct {
			Nodes []struct {
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"labels"`
	} `json:"content"`
}
