package entity

// IssueType represents the type of an issue in the OKR system
type IssueType string

const (
	IssueTypeObjective IssueType = "objective"
	IssueTypeKeyResult IssueType = "kr"
)

// Issue represents a GitHub issue in our OKR system
type Issue struct {
	Number int       `json:"number"`
	Title  string    `json:"title"`
	URL    string    `json:"url"`
	Type   IssueType `json:"type"`
	Body   string    `json:"body,omitempty"`
	State  string    `json:"state,omitempty"`
	Labels []string  `json:"labels,omitempty"`
}

// WeeklyUpdateStatus represents the status of a weekly update
type WeeklyUpdateStatus string

const (
	StatusOnTrack   WeeklyUpdateStatus = "on-track"
	StatusCaution   WeeklyUpdateStatus = "caution"
	StatusDelayed   WeeklyUpdateStatus = "delayed"
	StatusAtRisk    WeeklyUpdateStatus = "at-risk"
	StatusBlocked   WeeklyUpdateStatus = "blocked"
	StatusCompleted WeeklyUpdateStatus = "completed"
	StatusUnknown   WeeklyUpdateStatus = "unknown"
)

// WeeklyUpdate represents a weekly status update from issue comments
type WeeklyUpdate struct {
	Date    string             `json:"date"`
	Content string             `json:"content"`
	Author  string             `json:"author"`
	Status  WeeklyUpdateStatus `json:"status"`
}

// IssueWithUpdates represents an issue with its weekly updates and children
type IssueWithUpdates struct {
	Issue        Issue              `json:"issue"`
	LatestUpdate *WeeklyUpdate      `json:"latest_update,omitempty"`
	AllUpdates   []WeeklyUpdate     `json:"all_updates,omitempty"`
	ChildIssues  []IssueWithUpdates `json:"child_issues,omitempty"`
}

// IsObjective returns true if the issue is an objective
func (i *Issue) IsObjective() bool {
	return i.Type == IssueTypeObjective
}

// IsKeyResult returns true if the issue is a key result
func (i *Issue) IsKeyResult() bool {
	return i.Type == IssueTypeKeyResult
}

// HasLabel checks if the issue has a specific label
func (i *Issue) HasLabel(label string) bool {
	for _, l := range i.Labels {
		if l == label {
			return true
		}
	}
	return false
}

// HasAllLabels checks if the issue has all specified labels
func (i *Issue) HasAllLabels(requiredLabels []string) bool {
	if len(requiredLabels) == 0 {
		return true
	}

	foundLabels := make(map[string]bool)
	for _, label := range i.Labels {
		for _, required := range requiredLabels {
			if label == required {
				foundLabels[required] = true
			}
		}
	}

	for _, required := range requiredLabels {
		if !foundLabels[required] {
			return false
		}
	}

	return true
}

// GetLatestUpdateStatus returns the status of the latest update
func (i *IssueWithUpdates) GetLatestUpdateStatus() WeeklyUpdateStatus {
	if i.LatestUpdate != nil {
		return i.LatestUpdate.Status
	}
	return StatusUnknown
}

// GetActualStatus returns the status considering both weekly updates and GitHub issue state
// If an issue is marked as "completed" in comments but still open in GitHub, it should not be completed
func (i *IssueWithUpdates) GetActualStatus() WeeklyUpdateStatus {
	updateStatus := i.GetLatestUpdateStatus()
	
	// If the GitHub issue is closed, it should be completed regardless of update status
	if i.Issue.State == "closed" {
		return StatusCompleted
	}
	
	// If the update says "completed" but the GitHub issue is still open, 
	// it can't be truly completed - downgrade based on the actual update content
	if updateStatus == StatusCompleted && i.Issue.State == "open" {
		return StatusOnTrack // Downgrade to on-track since work is progressing but not finished
	}
	
	// For open issues, use the detected status from weekly updates
	return updateStatus
}

// GetKRStatus returns the KR status based on the latest weekly update symbol
// This prioritizes the status symbol from the most recent weekly update
func (i *IssueWithUpdates) GetKRStatus() WeeklyUpdateStatus {
	// If this is not a KR, use the original status
	if !i.Issue.IsKeyResult() {
		return i.GetActualStatus()
	}
	
	// If the GitHub issue is closed, it should be completed regardless of update status
	if i.Issue.State == "closed" {
		return StatusCompleted
	}
	
	// Look for the most recent weekly update with a valid status
	// Search through all updates to find the latest one with meaningful status
	for _, update := range i.AllUpdates {
		if update.Status != StatusUnknown {
			// Found a weekly update with a detected status symbol
			// If it says "completed" but GitHub issue is still open, downgrade to on-track
			if update.Status == StatusCompleted && i.Issue.State == "open" {
				return StatusOnTrack
			}
			// Return the detected status from the weekly update
			return update.Status
		}
	}
	
	// If no weekly updates have meaningful status, check latest update
	latestStatus := i.GetLatestUpdateStatus()
	if latestStatus != StatusUnknown {
		// If it says "completed" but GitHub issue is still open, downgrade to on-track
		if latestStatus == StatusCompleted && i.Issue.State == "open" {
			return StatusOnTrack
		}
		return latestStatus
	}
	
	// Default to unknown only if no weekly updates exist or none have status symbols
	return StatusUnknown
}

// GetObjectiveStatus returns the objective status based on its Key Results
// This aggregates the status of all child KRs to determine the objective's overall status
func (i *IssueWithUpdates) GetObjectiveStatus() WeeklyUpdateStatus {
	// If this is not an objective or has no child KRs, use the original status
	if !i.Issue.IsObjective() || len(i.ChildIssues) == 0 {
		return i.GetActualStatus()
	}
	
	// Count KR statuses
	var completed, blocked, delayed, atRisk, caution, onTrack, unknown int
	
	for _, kr := range i.ChildIssues {
		switch kr.GetKRStatus() {
		case StatusCompleted:
			completed++
		case StatusBlocked:
			blocked++
		case StatusDelayed:
			delayed++
		case StatusAtRisk:
			atRisk++
		case StatusCaution:
			caution++
		case StatusOnTrack:
			onTrack++
		case StatusUnknown:
			unknown++
		}
	}
	
	totalKRs := len(i.ChildIssues)
	
	// Determine objective status based on KR aggregation
	// Priority order: Blocked > Delayed > AtRisk > Caution > Completed > OnTrack > Unknown
	
	// If any KR is blocked, objective is blocked
	if blocked > 0 {
		return StatusBlocked
	}
	
	// If any KR is delayed, objective is delayed
	if delayed > 0 {
		return StatusDelayed
	}
	
	// If any KR is at risk, objective is at risk
	if atRisk > 0 {
		return StatusAtRisk
	}
	
	// If any KR is caution, objective is caution
	if caution > 0 {
		return StatusCaution
	}
	
	// If all KRs are completed, objective is completed
	if completed == totalKRs {
		return StatusCompleted
	}
	
	// If majority of KRs are completed (>= 50%), objective is on track
	if completed >= totalKRs/2 {
		return StatusOnTrack
	}
	
	// If we have a mix with on-track KRs, objective is on track
	if onTrack > 0 {
		return StatusOnTrack
	}
	
	// Default to unknown if all KRs are unknown
	return StatusUnknown
}
