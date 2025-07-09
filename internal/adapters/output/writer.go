package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github-okr-fetcher/internal/domain/entity"
	"github-okr-fetcher/internal/ports"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Writer implements the OutputWriter interface
type Writer struct {
	config *entity.Config
}

// NewWriter creates a new output writer
func NewWriter() *Writer {
	return &Writer{}
}

// NewWriterWithConfig creates a new output writer with configuration
func NewWriterWithConfig(config *entity.Config) *Writer {
	return &Writer{config: config}
}

// WriteMarkdown writes objectives as a markdown report
func (w *Writer) WriteMarkdown(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, filename string) error {
	content := w.formatAsMarkdown(objectives, projectInfo)
	return os.WriteFile(filename, []byte(content), 0644)
}

// WriteMarkdownWithAnalysis writes objectives as a markdown report with LiteLLM analysis
func (w *Writer) WriteMarkdownWithAnalysis(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, filename string, analysis string) error {
	content := w.formatAsMarkdownWithAnalysis(objectives, projectInfo, analysis)
	return os.WriteFile(filename, []byte(content), 0644)
}

// WriteJSON writes objectives as JSON
func (w *Writer) WriteJSON(objectives []*entity.IssueWithUpdates, filename string) error {
	data, err := json.MarshalIndent(objectives, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// WriteGoogleDocs writes objectives to markdown first, then converts to Google Docs
func (w *Writer) WriteGoogleDocs(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, documentURL, clientID, clientSecret string) error {
	// First, generate markdown content
	markdownContent := w.formatAsMarkdown(objectives, projectInfo)

	// Create markdown file in current directory
	markdownFile, err := w.createMarkdownFile(markdownContent, projectInfo)
	if err != nil {
		return fmt.Errorf("failed to create markdown file: %v", err)
	}

	fmt.Printf("📄 Generated markdown file: %s\n", markdownFile)
	fmt.Printf("📝 Converting to Google Docs document: %s\n", documentURL)

	// Create Google Docs client with OAuth2
	googleDocsClient, err := w.newGoogleDocsClientOAuth(clientID, clientSecret)
	if err != nil {
		return fmt.Errorf("failed to create Google Docs client: %v", err)
	}

	// Convert markdown to Google Docs
	if err := googleDocsClient.convertMarkdownToGoogleDocs(documentURL, markdownContent); err != nil {
		return fmt.Errorf("failed to convert markdown to Google Docs: %v", err)
	}

	fmt.Printf("✅ Successfully converted markdown to Google Docs\n")
	fmt.Printf("💡 Tip: You can also manually copy the content from: %s\n", markdownFile)
	return nil
}

// WriteGoogleDocsWithAnalysis writes objectives to markdown first with AI analysis, then converts to Google Docs
func (w *Writer) WriteGoogleDocsWithAnalysis(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, documentURL, clientID, clientSecret, analysis string) error {
	// First, generate markdown content with analysis
	markdownContent := w.formatAsMarkdownWithAnalysis(objectives, projectInfo, analysis)

	// Create markdown file in current directory
	markdownFile, err := w.createMarkdownFile(markdownContent, projectInfo)
	if err != nil {
		return fmt.Errorf("failed to create markdown file: %v", err)
	}

	fmt.Printf("📄 Generated markdown file with analysis: %s\n", markdownFile)
	fmt.Printf("📝 Converting to Google Docs document: %s\n", documentURL)

	// Create Google Docs client with OAuth2
	googleDocsClient, err := w.newGoogleDocsClientOAuth(clientID, clientSecret)
	if err != nil {
		return fmt.Errorf("failed to create Google Docs client: %v", err)
	}

	// Convert markdown to Google Docs
	if err := googleDocsClient.convertMarkdownToGoogleDocs(documentURL, markdownContent); err != nil {
		return fmt.Errorf("failed to convert markdown to Google Docs: %v", err)
	}

	fmt.Printf("✅ Successfully converted markdown with analysis to Google Docs\n")
	fmt.Printf("💡 Tip: You can also manually copy the content from: %s\n", markdownFile)
	return nil
}

// formatAsMarkdown formats objectives as markdown content
func (w *Writer) formatAsMarkdown(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo) string {
	var md strings.Builder

	// Header
	title := "OKR Report"
	if w.config != nil && w.config.Output.Title != "" {
		title = w.config.Output.Title
	}
	md.WriteString(fmt.Sprintf("# %s\n\n", title))

	// Project name
	projectName := "Project"
	if w.config != nil && w.config.Output.ProjectName != "" {
		projectName = w.config.Output.ProjectName
	}
	md.WriteString(fmt.Sprintf("📊 **Project**: [%s](https://github.com/orgs/%s/projects/%d/views/%d)\n\n",
		projectName, projectInfo.Owner, projectInfo.ProjectID, projectInfo.ViewID))
	md.WriteString(fmt.Sprintf("📅 **Generated**: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// If no objectives found, provide helpful information
	if len(objectives) == 0 {
		md.WriteString("## ⚠️ No OKR Data Found\n\n")
		md.WriteString("No issues were found that match the required criteria.\n\n")
		return md.String()
	}

	// Summary section
	md.WriteString("## 📈 Summary\n\n")

	totalObjectives := len(objectives)
	totalKRs := 0
	completedKRs := 0
	blockedKRs := 0
	delayedKRs := 0
	cautionKRs := 0
	atRiskKRs := 0
	onTrackKRs := 0

	for _, obj := range objectives {
		totalKRs += len(obj.ChildIssues)
		for _, kr := range obj.ChildIssues {
			switch kr.GetKRStatus() {
			case entity.StatusCompleted:
				completedKRs++
			case entity.StatusBlocked:
				blockedKRs++
			case entity.StatusDelayed:
				delayedKRs++
			case entity.StatusCaution:
				cautionKRs++
			case entity.StatusAtRisk:
				atRiskKRs++
			case entity.StatusOnTrack:
				onTrackKRs++
			}
		}
	}

	md.WriteString(fmt.Sprintf("- **Objectives**: %d\n", totalObjectives))
	md.WriteString(fmt.Sprintf("- **Key Results**: %d\n", totalKRs))
	md.WriteString(fmt.Sprintf("- ✅ **Completed**: %d\n", completedKRs))
	md.WriteString(fmt.Sprintf("- 🟢 **On Track**: %d\n", onTrackKRs))
	md.WriteString(fmt.Sprintf("- 🟡 **Caution**: %d\n", cautionKRs))
	md.WriteString(fmt.Sprintf("- ⚠️ **At Risk**: %d\n", atRiskKRs))
	md.WriteString(fmt.Sprintf("- 🔴 **Delayed**: %d\n", delayedKRs))
	md.WriteString(fmt.Sprintf("- 🚫 **Blocked**: %d\n\n", blockedKRs))

	// Progress bar
	if totalKRs > 0 {
		completionRate := float64(completedKRs) / float64(totalKRs) * 100
		md.WriteString(fmt.Sprintf("**Overall Progress**: %.1f%% (%d/%d completed)\n\n", completionRate, completedKRs, totalKRs))

		// Visual progress bar
		progressBars := int(completionRate / 10)
		md.WriteString("```\n")
		md.WriteString("Progress: [")
		for i := 0; i < 10; i++ {
			if i < progressBars {
				md.WriteString("█")
			} else {
				md.WriteString("░")
			}
		}
		md.WriteString(fmt.Sprintf("] %.1f%%\n", completionRate))
		md.WriteString("```\n\n")
	}

	md.WriteString("---\n\n")

	// Objectives and KRs
	md.WriteString("## 🎯 Objectives & Key Results\n\n")

	for i, obj := range objectives {
		// Objective header - use status derived from KRs
		objStatus := obj.GetObjectiveStatus()
		indicator := w.getStatusIndicator(objStatus)

		md.WriteString(fmt.Sprintf("### %d. %s %s\n", i+1, indicator.Icon, obj.Issue.Title))
		md.WriteString(fmt.Sprintf("**Issue**: [#%d](%s) | **Status**: %s\n\n",
			obj.Issue.Number, obj.Issue.URL, indicator.Status))

		// Two latest updates for the objective
		w.formatTwoLatestUpdates(&md, obj)

		// Key Results
		if len(obj.ChildIssues) > 0 {
			md.WriteString("#### 📋 Key Results:\n\n")

			for j, kr := range obj.ChildIssues {
				krStatus := kr.GetKRStatus()
				krIndicator := w.getStatusIndicator(krStatus)

				md.WriteString(fmt.Sprintf("%d.%d. %s **[%s](%s)**\n",
					i+1, j+1, krIndicator.Icon, kr.Issue.Title, kr.Issue.URL))
				md.WriteString(fmt.Sprintf("   - **Issue**: [#%d](%s)\n",
					kr.Issue.Number, kr.Issue.URL))
				md.WriteString(fmt.Sprintf("   - **Status**: %s\n", krIndicator.Status))

				// Add weekly updates section for KR
				w.formatWeeklyUpdatesForKR(&md, kr, i+1, j+1)
				md.WriteString("\n")
			}
		}

		md.WriteString("---\n\n")
	}

	// Footer
	md.WriteString("## 📝 Notes\n\n")
	md.WriteString("- This report is automatically generated from GitHub issues and comments\n")
	md.WriteString("- Status indicators are detected from weekly update comments\n")
	md.WriteString("- Click on issue links to view full details and discussions\n")
	md.WriteString(fmt.Sprintf("- Last updated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	return md.String()
}

// formatAsMarkdownWithAnalysis formats objectives as markdown content with LiteLLM analysis
func (w *Writer) formatAsMarkdownWithAnalysis(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, analysis string) string {
	var md strings.Builder

	// Header
	title := "OKR Report"
	if w.config != nil && w.config.Output.Title != "" {
		title = w.config.Output.Title
	}
	md.WriteString(fmt.Sprintf("# %s\n\n", title))

	// Project name
	projectName := "Project"
	if w.config != nil && w.config.Output.ProjectName != "" {
		projectName = w.config.Output.ProjectName
	}
	md.WriteString(fmt.Sprintf("📊 **Project**: [%s](https://github.com/orgs/%s/projects/%d/views/%d)\n\n",
		projectName, projectInfo.Owner, projectInfo.ProjectID, projectInfo.ViewID))
	md.WriteString(fmt.Sprintf("📅 **Generated**: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// AI Analysis Section (if available)
	if analysis != "" {
		md.WriteString("## 🤖 AI Analysis\n\n")
		md.WriteString(analysis)
		md.WriteString("\n\n")
		md.WriteString("---\n\n")
	}

	// If no objectives found, provide helpful information
	if len(objectives) == 0 {
		md.WriteString("## ⚠️ No OKR Data Found\n\n")
		md.WriteString("No issues were found that match the required criteria.\n\n")
		return md.String()
	}

	// Summary section
	md.WriteString("## 📈 Summary\n\n")

	totalObjectives := len(objectives)
	totalKRs := 0
	completedKRs := 0
	blockedKRs := 0
	delayedKRs := 0
	cautionKRs := 0
	atRiskKRs := 0
	onTrackKRs := 0

	for _, obj := range objectives {
		totalKRs += len(obj.ChildIssues)
		for _, kr := range obj.ChildIssues {
			switch kr.GetKRStatus() {
			case entity.StatusCompleted:
				completedKRs++
			case entity.StatusBlocked:
				blockedKRs++
			case entity.StatusDelayed:
				delayedKRs++
			case entity.StatusCaution:
				cautionKRs++
			case entity.StatusAtRisk:
				atRiskKRs++
			case entity.StatusOnTrack:
				onTrackKRs++
			}
		}
	}

	md.WriteString(fmt.Sprintf("- **Objectives**: %d\n", totalObjectives))
	md.WriteString(fmt.Sprintf("- **Key Results**: %d\n", totalKRs))
	md.WriteString(fmt.Sprintf("- ✅ **Completed**: %d\n", completedKRs))
	md.WriteString(fmt.Sprintf("- 🟢 **On Track**: %d\n", onTrackKRs))
	md.WriteString(fmt.Sprintf("- 🟡 **Caution**: %d\n", cautionKRs))
	md.WriteString(fmt.Sprintf("- ⚠️ **At Risk**: %d\n", atRiskKRs))
	md.WriteString(fmt.Sprintf("- 🔴 **Delayed**: %d\n", delayedKRs))
	md.WriteString(fmt.Sprintf("- 🚫 **Blocked**: %d\n\n", blockedKRs))

	// Progress bar
	if totalKRs > 0 {
		completionRate := float64(completedKRs) / float64(totalKRs) * 100
		md.WriteString(fmt.Sprintf("**Overall Progress**: %.1f%% (%d/%d completed)\n\n", completionRate, completedKRs, totalKRs))

		// Visual progress bar
		progressBars := int(completionRate / 10)
		md.WriteString("```\n")
		md.WriteString("Progress: [")
		for i := 0; i < 10; i++ {
			if i < progressBars {
				md.WriteString("█")
			} else {
				md.WriteString("░")
			}
		}
		md.WriteString(fmt.Sprintf("] %.1f%%\n", completionRate))
		md.WriteString("```\n\n")
	}

	md.WriteString("---\n\n")

	// Objectives and KRs
	md.WriteString("## 🎯 Objectives & Key Results\n\n")

	for i, obj := range objectives {
		// Objective header - use status derived from KRs
		objStatus := obj.GetObjectiveStatus()
		indicator := w.getStatusIndicator(objStatus)

		md.WriteString(fmt.Sprintf("### %d. %s %s\n", i+1, indicator.Icon, obj.Issue.Title))
		md.WriteString(fmt.Sprintf("**Issue**: [#%d](%s) | **Status**: %s\n\n",
			obj.Issue.Number, obj.Issue.URL, indicator.Status))

		// Two latest updates for the objective
		w.formatTwoLatestUpdates(&md, obj)

		// Key Results
		if len(obj.ChildIssues) > 0 {
			md.WriteString("#### 📋 Key Results:\n\n")

			for j, kr := range obj.ChildIssues {
				krStatus := kr.GetKRStatus()
				krIndicator := w.getStatusIndicator(krStatus)

				md.WriteString(fmt.Sprintf("%d.%d. %s **[%s](%s)**\n",
					i+1, j+1, krIndicator.Icon, kr.Issue.Title, kr.Issue.URL))
				md.WriteString(fmt.Sprintf("   - **Issue**: [#%d](%s)\n",
					kr.Issue.Number, kr.Issue.URL))
				md.WriteString(fmt.Sprintf("   - **Status**: %s\n", krIndicator.Status))

				// Add weekly updates section for KR
				w.formatWeeklyUpdatesForKR(&md, kr, i+1, j+1)
				md.WriteString("\n")
			}
		}

		md.WriteString("---\n\n")
	}

	// Footer
	md.WriteString("## 📝 Notes\n\n")
	md.WriteString("- This report is automatically generated from GitHub issues and comments\n")
	md.WriteString("- Status indicators are detected from weekly update comments\n")
	md.WriteString("- Click on issue links to view full details and discussions\n")
	if analysis != "" {
		md.WriteString("- AI analysis is provided by LiteLLM for insights and recommendations\n")
	}
	md.WriteString(fmt.Sprintf("- Last updated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	return md.String()
}

// formatTwoLatestUpdates formats the two most recent weekly updates in a pretty format
func (w *Writer) formatTwoLatestUpdates(md *strings.Builder, issue *entity.IssueWithUpdates) {
	// Get all updates and take the two most recent
	updates := issue.AllUpdates
	if len(updates) == 0 {
		return
	}

	// Take up to 2 most recent updates (they're already sorted by date descending)
	maxUpdates := 2
	if len(updates) < maxUpdates {
		maxUpdates = len(updates)
	}

	for i := 0; i < maxUpdates; i++ {
		update := updates[i]

		// Format the update header
		updatePrefix := "**Latest Update**"
		if i == 1 {
			updatePrefix = "**Previous Update**"
		}

		md.WriteString(fmt.Sprintf("%s (%s by @%s):\n", updatePrefix, update.Date, update.Author))

		// Extract and format the content with better presentation
		summary := w.formatWeeklyUpdateContent(update.Content)
		md.WriteString(summary)
		md.WriteString("\n\n")
	}
}

// formatWeeklyUpdateContent displays the full weekly update content
func (w *Writer) formatWeeklyUpdateContent(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	result.WriteString("```\n")

	// Display the full content with minimal filtering
	for _, line := range lines {
		// Skip only the weekly update header line if it exists
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "# weekly update") {
			continue
		}

		// Include all other content as-is
		result.WriteString(line + "\n")
	}

	result.WriteString("```")

	return result.String()
}

// formatTwoLatestUpdatesForGoogleDocsRich formats the two most recent weekly updates for Google Docs with rich formatting
func (w *Writer) formatTwoLatestUpdatesForGoogleDocsRich(doc *strings.Builder, issue *entity.IssueWithUpdates) {
	// Get all updates and take the two most recent
	updates := issue.AllUpdates
	if len(updates) == 0 {
		return
	}

	// Take up to 2 most recent updates (they're already sorted by date descending)
	maxUpdates := 2
	if len(updates) < maxUpdates {
		maxUpdates = len(updates)
	}

	for i := 0; i < maxUpdates; i++ {
		update := updates[i]

		// Format the update header - match markdown style
		updatePrefix := "Latest Update"
		if i == 1 {
			updatePrefix = "Previous Update"
		}

		doc.WriteString(fmt.Sprintf("%s (%s by @%s):\n", updatePrefix, update.Date, update.Author))

		// Extract and format the content with better presentation - preserve markdown structure
		summary := w.formatWeeklyUpdateContentForGoogleDocsRich(update.Content)
		doc.WriteString(summary)
		doc.WriteString("\n\n")
	}
}

// formatWeeklyUpdateContentForGoogleDocsRich displays the full weekly update content for Google Docs with rich formatting
func (w *Writer) formatWeeklyUpdateContentForGoogleDocsRich(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	// Display the full content with minimal filtering for Google Docs but preserve structure
	for _, line := range lines {
		// Skip only the weekly update header line if it exists
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "# weekly update") {
			continue
		}

		// Include all other content as-is with proper formatting for Google Docs
		result.WriteString(line + "\n")
	}

	return result.String()
}

// formatWeeklyUpdatesForKRGoogleDocsRich formats weekly updates for a specific KR in Google Docs with rich formatting
func (w *Writer) formatWeeklyUpdatesForKRGoogleDocsRich(doc *strings.Builder, kr entity.IssueWithUpdates, objNum, krNum int) {
	// Get all weekly updates
	weeklyUpdates := w.getWeeklyUpdates(kr.AllUpdates)

	if len(weeklyUpdates) == 0 {
		return
	}

	// Take up to 2 most recent weekly updates
	maxUpdates := 2
	if len(weeklyUpdates) < maxUpdates {
		maxUpdates = len(weeklyUpdates)
	}

	doc.WriteString("   - Weekly Updates:\n")

	for i := 0; i < maxUpdates; i++ {
		update := weeklyUpdates[i]

		// Format the update header - match markdown style
		updateLabel := "Latest"
		if i == 1 {
			updateLabel = "Previous"
		}

		doc.WriteString(fmt.Sprintf("     - %s (%s by @%s):\n", updateLabel, update.Date, update.Author))

		// Parse and format the content nicely for Google Docs - use rich formatting like markdown
		formattedContent := w.formatWeeklyUpdateContentPrettyGoogleDocsRich(update.Content)
		doc.WriteString(formattedContent)
	}
}

// formatWeeklyUpdateContentPrettyGoogleDocsRich formats weekly update content for Google Docs with rich markdown-like formatting
func (w *Writer) formatWeeklyUpdateContentPrettyGoogleDocsRich(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	// Parse content into structured sections (same logic as markdown version but with rich formatting)
	var currentSection string
	var statusAssessment map[string]string
	var goals []string
	var keyPoints []string
	var doneItems []string
	var inProgressItems []string
	var notes []string

	inTable := false
	currentKey := ""

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		lowerLine := strings.ToLower(trimmedLine)

		// Skip empty lines and weekly update headers
		if trimmedLine == "" || strings.HasPrefix(lowerLine, "# weekly update") {
			continue
		}

		// Handle HTML table parsing for status assessment
		if strings.Contains(lowerLine, "<table>") {
			inTable = true
			statusAssessment = make(map[string]string)
			continue
		}
		if strings.Contains(lowerLine, "</table>") {
			inTable = false
			currentKey = ""
			continue
		}
		if inTable {
			if strings.Contains(lowerLine, "<th>") {
				// Extract table header
				currentKey = w.extractTextFromHTML(trimmedLine)
			} else if strings.Contains(lowerLine, "<span>") && currentKey != "" {
				// Extract table value
				value := w.extractTextFromHTML(trimmedLine)
				if value != "" && !strings.Contains(value, "Choose one") {
					statusAssessment[currentKey] = value
				}
			}
			continue
		}

		// Identify sections
		if strings.HasPrefix(trimmedLine, "###") || strings.HasPrefix(trimmedLine, "##") {
			sectionTitle := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmedLine, "###"), "##"))
			sectionTitle = strings.TrimSpace(strings.TrimPrefix(sectionTitle, "#"))
			currentSection = strings.ToLower(sectionTitle)
			continue
		}

		// Collect content based on current section
		if currentSection != "" && trimmedLine != "" && !strings.HasPrefix(trimmedLine, "#") {
			switch {
			case strings.Contains(currentSection, "goal"):
				goals = append(goals, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "key points") || strings.Contains(currentSection, "💡"):
				keyPoints = append(keyPoints, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "done") || strings.Contains(currentSection, "🎉"):
				doneItems = append(doneItems, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "progress") || strings.Contains(currentSection, "todo") || strings.Contains(currentSection, "🏃"):
				inProgressItems = append(inProgressItems, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "note") || strings.Contains(currentSection, "blocker") || strings.Contains(currentSection, "🗒"):
				notes = append(notes, w.cleanBulletPoint(trimmedLine))
			}
		}
	}

	// Format the output in a clean, structured way for Google Docs - match markdown style
	result.WriteString("\n")

	// Status Assessment (if available)
	if len(statusAssessment) > 0 {
		result.WriteString("       📊 Status:\n")
		for key, value := range statusAssessment {
			result.WriteString(fmt.Sprintf("       - %s: %s\n", key, value))
		}
		result.WriteString("\n")
	}

	// Goals (if available)
	if len(goals) > 0 {
		result.WriteString("       🎯 Goals:\n")
		for _, goal := range goals {
			if goal != "" {
				result.WriteString(fmt.Sprintf("       - %s\n", goal))
			}
		}
		result.WriteString("\n")
	}

	// Key points first (most important)
	if len(keyPoints) > 0 {
		result.WriteString("       💡 Key Points:\n")
		for _, point := range keyPoints {
			if point != "" && len(point) > 5 {
				result.WriteString(fmt.Sprintf("       - %s\n", point))
			}
		}
		result.WriteString("\n")
	}

	// Done items
	if len(doneItems) > 0 {
		result.WriteString("       ✅ Completed:\n")
		for _, item := range doneItems {
			if item != "" {
				result.WriteString(fmt.Sprintf("       - %s\n", item))
			}
		}
		result.WriteString("\n")
	}

	// In progress items
	if len(inProgressItems) > 0 {
		result.WriteString("       🏃 In Progress:\n")
		for _, item := range inProgressItems {
			if item != "" {
				result.WriteString(fmt.Sprintf("       - %s\n", item))
			}
		}
		result.WriteString("\n")
	}

	// Notes and blockers
	if len(notes) > 0 {
		result.WriteString("       🗒 Notes:\n")
		for _, note := range notes {
			if note != "" {
				result.WriteString(fmt.Sprintf("       - %s\n", note))
			}
		}
		result.WriteString("\n")
	}

	return result.String()
}

// formatTwoLatestUpdatesForGoogleDocs formats the two most recent weekly updates for Google Docs
func (w *Writer) formatTwoLatestUpdatesForGoogleDocs(doc *strings.Builder, issue *entity.IssueWithUpdates) {
	// Get all updates and take the two most recent
	updates := issue.AllUpdates
	if len(updates) == 0 {
		return
	}

	// Take up to 2 most recent updates (they're already sorted by date descending)
	maxUpdates := 2
	if len(updates) < maxUpdates {
		maxUpdates = len(updates)
	}

	for i := 0; i < maxUpdates; i++ {
		update := updates[i]

		// Format the update header
		updatePrefix := "Latest Update"
		if i == 1 {
			updatePrefix = "Previous Update"
		}

		doc.WriteString(fmt.Sprintf("%s (%s by @%s):\n", updatePrefix, update.Date, update.Author))

		// Extract and format the content with better presentation
		summary := w.formatWeeklyUpdateContentForGoogleDocs(update.Content)
		doc.WriteString(summary)
		doc.WriteString("\n\n")
	}
}

// formatWeeklyUpdateContentForGoogleDocs displays the full weekly update content for Google Docs
func (w *Writer) formatWeeklyUpdateContentForGoogleDocs(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	// Display the full content with minimal filtering for Google Docs
	for _, line := range lines {
		// Skip only the weekly update header line if it exists
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "# weekly update") {
			continue
		}

		// Include all other content as-is with proper indentation for Google Docs
		result.WriteString("  " + line + "\n")
	}

	return result.String()
}

// formatWeeklyUpdatesForKR formats weekly updates for a specific KR
func (w *Writer) formatWeeklyUpdatesForKR(md *strings.Builder, kr entity.IssueWithUpdates, objNum, krNum int) {
	// Get all weekly updates
	weeklyUpdates := w.getWeeklyUpdates(kr.AllUpdates)

	if len(weeklyUpdates) == 0 {
		return
	}

	// Take up to 2 most recent weekly updates
	maxUpdates := 2
	if len(weeklyUpdates) < maxUpdates {
		maxUpdates = len(weeklyUpdates)
	}

	md.WriteString("   - **Weekly Updates**:\n")

	for i := 0; i < maxUpdates; i++ {
		update := weeklyUpdates[i]

		// Format the update header
		updateLabel := "Latest"
		if i == 1 {
			updateLabel = "Previous"
		}

		md.WriteString(fmt.Sprintf("     - **%s** (%s by @%s):\n", updateLabel, update.Date, update.Author))

		// Parse and format the content nicely
		formattedContent := w.formatWeeklyUpdateContentPretty(update.Content)
		md.WriteString(formattedContent)
	}
}

// getWeeklyUpdates filters updates to only include those with "weekly update yyyy-mm-dd" pattern
func (w *Writer) getWeeklyUpdates(allUpdates []entity.WeeklyUpdate) []entity.WeeklyUpdate {
	var weeklyUpdates []entity.WeeklyUpdate

	// Regular expression to match "weekly update yyyy-mm-dd" pattern
	weeklyPattern := `(?i)weekly\s+update\s+\d{4}-\d{2}-\d{2}`

	for _, update := range allUpdates {
		// Check if the content contains the weekly update pattern
		if matched, _ := regexp.MatchString(weeklyPattern, update.Content); matched {
			weeklyUpdates = append(weeklyUpdates, update)
		}
	}

	// Sort by date descending (most recent first) - they should already be sorted
	// but let's ensure it for safety
	sort.Slice(weeklyUpdates, func(i, j int) bool {
		return weeklyUpdates[i].Date > weeklyUpdates[j].Date
	})

	return weeklyUpdates
}

// formatWeeklyUpdateContentPretty formats weekly update content in a clean, structured way
func (w *Writer) formatWeeklyUpdateContentPretty(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	// Parse content into structured sections
	var currentSection string
	var statusAssessment map[string]string
	var goals []string
	var keyPoints []string
	var doneItems []string
	var inProgressItems []string
	var notes []string

	inTable := false
	currentKey := ""

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		lowerLine := strings.ToLower(trimmedLine)

		// Skip empty lines and weekly update headers
		if trimmedLine == "" || strings.HasPrefix(lowerLine, "# weekly update") {
			continue
		}

		// Handle HTML table parsing for status assessment
		if strings.Contains(lowerLine, "<table>") {
			inTable = true
			statusAssessment = make(map[string]string)
			continue
		}
		if strings.Contains(lowerLine, "</table>") {
			inTable = false
			currentKey = ""
			continue
		}
		if inTable {
			if strings.Contains(lowerLine, "<th>") {
				// Extract table header
				currentKey = w.extractTextFromHTML(trimmedLine)
			} else if strings.Contains(lowerLine, "<span>") && currentKey != "" {
				// Extract table value
				value := w.extractTextFromHTML(trimmedLine)
				if value != "" && !strings.Contains(value, "Choose one") {
					statusAssessment[currentKey] = value
				}
			}
			continue
		}

		// Identify sections
		if strings.HasPrefix(trimmedLine, "###") || strings.HasPrefix(trimmedLine, "##") {
			sectionTitle := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmedLine, "###"), "##"))
			sectionTitle = strings.TrimSpace(strings.TrimPrefix(sectionTitle, "#"))
			currentSection = strings.ToLower(sectionTitle)
			continue
		}

		// Collect content based on current section
		if currentSection != "" && trimmedLine != "" && !strings.HasPrefix(trimmedLine, "#") {
			switch {
			case strings.Contains(currentSection, "goal"):
				goals = append(goals, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "key points") || strings.Contains(currentSection, "💡"):
				keyPoints = append(keyPoints, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "done") || strings.Contains(currentSection, "🎉"):
				doneItems = append(doneItems, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "progress") || strings.Contains(currentSection, "todo") || strings.Contains(currentSection, "🏃"):
				inProgressItems = append(inProgressItems, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "note") || strings.Contains(currentSection, "blocker") || strings.Contains(currentSection, "🗒"):
				notes = append(notes, w.cleanBulletPoint(trimmedLine))
			}
		}
	}

	// Format the output in a clean, structured way
	result.WriteString("\n")

	// Status Assessment (if available)
	if len(statusAssessment) > 0 {
		result.WriteString("       **📊 Status:**\n")
		for key, value := range statusAssessment {
			result.WriteString(fmt.Sprintf("       - %s: %s\n", key, value))
		}
		result.WriteString("\n")
	}

	// Goals (if available)
	if len(goals) > 0 {
		result.WriteString("       **🎯 Goals:**\n")
		for _, goal := range goals {
			if goal != "" {
				result.WriteString(fmt.Sprintf("       - %s\n", goal))
			}
		}
		result.WriteString("\n")
	}

	// Key points first (most important)
	if len(keyPoints) > 0 {
		result.WriteString("       **💡 Key Points:**\n")
		for _, point := range keyPoints {
			if point != "" && len(point) > 5 {
				result.WriteString(fmt.Sprintf("       - %s\n", point))
			}
		}
		result.WriteString("\n")
	}

	// Done items
	if len(doneItems) > 0 {
		result.WriteString("       **✅ Completed:**\n")
		for _, item := range doneItems {
			if item != "" {
				result.WriteString(fmt.Sprintf("       - %s\n", item))
			}
		}
		result.WriteString("\n")
	}

	// In progress items
	if len(inProgressItems) > 0 {
		result.WriteString("       **🏃 In Progress:**\n")
		for _, item := range inProgressItems {
			if item != "" {
				result.WriteString(fmt.Sprintf("       - %s\n", item))
			}
		}
		result.WriteString("\n")
	}

	// Notes and blockers
	if len(notes) > 0 {
		result.WriteString("       **🗒 Notes:**\n")
		for _, note := range notes {
			if note != "" {
				result.WriteString(fmt.Sprintf("       - %s\n", note))
			}
		}
		result.WriteString("\n")
	}

	return result.String()
}

// extractTextFromHTML extracts text content from simple HTML tags
func (w *Writer) extractTextFromHTML(htmlLine string) string {
	// Remove HTML tags and get the text content
	text := htmlLine
	text = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	return text
}

// cleanBulletPoint cleans up bullet point formatting and extracts meaningful content
func (w *Writer) cleanBulletPoint(line string) string {
	// Remove common bullet point markers
	cleaned := strings.TrimSpace(line)
	cleaned = strings.TrimPrefix(cleaned, "- ")
	cleaned = strings.TrimPrefix(cleaned, "* ")
	cleaned = strings.TrimPrefix(cleaned, "+ ")
	cleaned = strings.TrimPrefix(cleaned, "• ")
	cleaned = strings.TrimPrefix(cleaned, "→ ")

	// Remove markdown formatting
	cleaned = strings.TrimPrefix(cleaned, "**")
	cleaned = strings.TrimSuffix(cleaned, "**")
	cleaned = strings.TrimSpace(cleaned)

	// Skip lines that are just usernames, URLs, or HTML, or too short
	if strings.HasPrefix(cleaned, "@") ||
		strings.HasPrefix(cleaned, "http") ||
		strings.Contains(cleaned, "<") ||
		strings.Contains(cleaned, ">") ||
		len(cleaned) < 3 {
		return ""
	}

	return cleaned
}

// formatWeeklyUpdatesForKRGoogleDocs formats weekly updates for a specific KR in Google Docs format
func (w *Writer) formatWeeklyUpdatesForKRGoogleDocs(doc *strings.Builder, kr entity.IssueWithUpdates, objNum, krNum int) {
	// Get all weekly updates
	weeklyUpdates := w.getWeeklyUpdates(kr.AllUpdates)

	if len(weeklyUpdates) == 0 {
		return
	}

	// Take up to 2 most recent weekly updates
	maxUpdates := 2
	if len(weeklyUpdates) < maxUpdates {
		maxUpdates = len(weeklyUpdates)
	}

	doc.WriteString("     Weekly Updates:\n")

	for i := 0; i < maxUpdates; i++ {
		update := weeklyUpdates[i]

		// Format the update header
		updateLabel := "Latest"
		if i == 1 {
			updateLabel = "Previous"
		}

		doc.WriteString(fmt.Sprintf("       %s (%s by @%s):\n", updateLabel, update.Date, update.Author))

		// Parse and format the content nicely for Google Docs
		formattedContent := w.formatWeeklyUpdateContentPrettyGoogleDocs(update.Content)
		doc.WriteString(formattedContent)
	}
}

// formatWeeklyUpdateContentPrettyGoogleDocs formats weekly update content for Google Docs in a clean, structured way
func (w *Writer) formatWeeklyUpdateContentPrettyGoogleDocs(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	// Parse content into structured sections (same logic as markdown version)
	var currentSection string
	var statusAssessment map[string]string
	var goals []string
	var keyPoints []string
	var doneItems []string
	var inProgressItems []string
	var notes []string

	inTable := false
	currentKey := ""

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		lowerLine := strings.ToLower(trimmedLine)

		// Skip empty lines and weekly update headers
		if trimmedLine == "" || strings.HasPrefix(lowerLine, "# weekly update") {
			continue
		}

		// Handle HTML table parsing for status assessment
		if strings.Contains(lowerLine, "<table>") {
			inTable = true
			statusAssessment = make(map[string]string)
			continue
		}
		if strings.Contains(lowerLine, "</table>") {
			inTable = false
			currentKey = ""
			continue
		}
		if inTable {
			if strings.Contains(lowerLine, "<th>") {
				// Extract table header
				currentKey = w.extractTextFromHTML(trimmedLine)
			} else if strings.Contains(lowerLine, "<span>") && currentKey != "" {
				// Extract table value
				value := w.extractTextFromHTML(trimmedLine)
				if value != "" && !strings.Contains(value, "Choose one") {
					statusAssessment[currentKey] = value
				}
			}
			continue
		}

		// Identify sections
		if strings.HasPrefix(trimmedLine, "###") || strings.HasPrefix(trimmedLine, "##") {
			sectionTitle := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmedLine, "###"), "##"))
			sectionTitle = strings.TrimSpace(strings.TrimPrefix(sectionTitle, "#"))
			currentSection = strings.ToLower(sectionTitle)
			continue
		}

		// Collect content based on current section
		if currentSection != "" && trimmedLine != "" && !strings.HasPrefix(trimmedLine, "#") {
			switch {
			case strings.Contains(currentSection, "goal"):
				goals = append(goals, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "key points") || strings.Contains(currentSection, "💡"):
				keyPoints = append(keyPoints, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "done") || strings.Contains(currentSection, "🎉"):
				doneItems = append(doneItems, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "progress") || strings.Contains(currentSection, "todo") || strings.Contains(currentSection, "🏃"):
				inProgressItems = append(inProgressItems, w.cleanBulletPoint(trimmedLine))
			case strings.Contains(currentSection, "note") || strings.Contains(currentSection, "blocker") || strings.Contains(currentSection, "🗒"):
				notes = append(notes, w.cleanBulletPoint(trimmedLine))
			}
		}
	}

	// Format the output in a clean, structured way for Google Docs
	result.WriteString("\n")

	// Status Assessment (if available)
	if len(statusAssessment) > 0 {
		result.WriteString("         Status:\n")
		for key, value := range statusAssessment {
			result.WriteString(fmt.Sprintf("         - %s: %s\n", key, value))
		}
		result.WriteString("\n")
	}

	// Goals (if available)
	if len(goals) > 0 {
		result.WriteString("         Goals:\n")
		for _, goal := range goals {
			if goal != "" {
				result.WriteString(fmt.Sprintf("         - %s\n", goal))
			}
		}
		result.WriteString("\n")
	}

	// Key points first (most important)
	if len(keyPoints) > 0 {
		result.WriteString("         Key Points:\n")
		for _, point := range keyPoints {
			if point != "" && len(point) > 5 {
				result.WriteString(fmt.Sprintf("         - %s\n", point))
			}
		}
		result.WriteString("\n")
	}

	// Done items
	if len(doneItems) > 0 {
		result.WriteString("         Completed:\n")
		for _, item := range doneItems {
			if item != "" {
				result.WriteString(fmt.Sprintf("         - %s\n", item))
			}
		}
		result.WriteString("\n")
	}

	// In progress items
	if len(inProgressItems) > 0 {
		result.WriteString("         In Progress:\n")
		for _, item := range inProgressItems {
			if item != "" {
				result.WriteString(fmt.Sprintf("         - %s\n", item))
			}
		}
		result.WriteString("\n")
	}

	// Notes and blockers
	if len(notes) > 0 {
		result.WriteString("         Notes:\n")
		for _, note := range notes {
			if note != "" {
				result.WriteString(fmt.Sprintf("         - %s\n", note))
			}
		}
		result.WriteString("\n")
	}

	return result.String()
}

// formatAsGoogleDocs formats objectives as Google Docs compatible plain text with rich formatting
func (w *Writer) formatAsGoogleDocs(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo) string {
	var doc strings.Builder

	// Header - match markdown format exactly
	title := "OKR Report"
	if w.config != nil && w.config.Output.Title != "" {
		title = w.config.Output.Title
	}
	doc.WriteString(fmt.Sprintf("# %s\n\n", title))

	// Project name
	projectName := "Project"
	if w.config != nil && w.config.Output.ProjectName != "" {
		projectName = w.config.Output.ProjectName
	}
	doc.WriteString(fmt.Sprintf("📊 Project: %s (%s)\n\n",
		projectName, fmt.Sprintf("https://github.com/orgs/%s/projects/%d/views/%d", projectInfo.Owner, projectInfo.ProjectID, projectInfo.ViewID)))
	doc.WriteString(fmt.Sprintf("📅 Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// If no objectives found
	if len(objectives) == 0 {
		doc.WriteString("## ⚠️ No OKR Data Found\n\n")
		doc.WriteString("No issues were found that match the required criteria.\n\n")
		return doc.String()
	}

	// Summary section - match markdown format
	doc.WriteString("## 📈 Summary\n\n")

	totalObjectives := len(objectives)
	totalKRs := 0
	completedKRs := 0
	blockedKRs := 0
	delayedKRs := 0
	cautionKRs := 0
	atRiskKRs := 0
	onTrackKRs := 0

	for _, obj := range objectives {
		totalKRs += len(obj.ChildIssues)
		for _, kr := range obj.ChildIssues {
			switch kr.GetKRStatus() {
			case entity.StatusCompleted:
				completedKRs++
			case entity.StatusBlocked:
				blockedKRs++
			case entity.StatusDelayed:
				delayedKRs++
			case entity.StatusCaution:
				cautionKRs++
			case entity.StatusAtRisk:
				atRiskKRs++
			case entity.StatusOnTrack:
				onTrackKRs++
			}
		}
	}

	doc.WriteString(fmt.Sprintf("- Objectives: %d\n", totalObjectives))
	doc.WriteString(fmt.Sprintf("- Key Results: %d\n", totalKRs))
	doc.WriteString(fmt.Sprintf("- ✅ Completed: %d\n", completedKRs))
	doc.WriteString(fmt.Sprintf("- 🟢 On Track: %d\n", onTrackKRs))
	doc.WriteString(fmt.Sprintf("- 🟡 Caution: %d\n", cautionKRs))
	doc.WriteString(fmt.Sprintf("- ⚠️ At Risk: %d\n", atRiskKRs))
	doc.WriteString(fmt.Sprintf("- 🔴 Delayed: %d\n", delayedKRs))
	doc.WriteString(fmt.Sprintf("- 🚫 Blocked: %d\n\n", blockedKRs))

	// Progress bar - match markdown format
	if totalKRs > 0 {
		completionRate := float64(completedKRs) / float64(totalKRs) * 100
		doc.WriteString(fmt.Sprintf("Overall Progress: %.1f%% (%d/%d completed)\n\n", completionRate, completedKRs, totalKRs))

		// Visual progress bar - same as markdown
		progressBars := int(completionRate / 10)
		doc.WriteString("Progress: [")
		for i := 0; i < 10; i++ {
			if i < progressBars {
				doc.WriteString("█")
			} else {
				doc.WriteString("░")
			}
		}
		doc.WriteString(fmt.Sprintf("] %.1f%%\n\n", completionRate))
	}

	doc.WriteString("---\n\n")

	// Objectives and KRs - match markdown format exactly
	doc.WriteString("## 🎯 Objectives & Key Results\n\n")

	for i, obj := range objectives {
		// Objective header - use status derived from KRs
		objStatus := obj.GetObjectiveStatus()
		indicator := w.getStatusIndicator(objStatus)

		doc.WriteString(fmt.Sprintf("### %d. %s %s\n", i+1, indicator.Icon, obj.Issue.Title))
		doc.WriteString(fmt.Sprintf("Issue: #%d (%s) | Status: %s\n\n",
			obj.Issue.Number, obj.Issue.URL, indicator.Status))

		// Two latest updates for the objective - use markdown-style formatting
		w.formatTwoLatestUpdatesForGoogleDocsRich(&doc, obj)

		// Key Results
		if len(obj.ChildIssues) > 0 {
			doc.WriteString("#### 📋 Key Results:\n\n")

			for j, kr := range obj.ChildIssues {
				krStatus := kr.GetKRStatus()
				krIndicator := w.getStatusIndicator(krStatus)

				doc.WriteString(fmt.Sprintf("%d.%d. %s %s (%s)\n",
					i+1, j+1, krIndicator.Icon, kr.Issue.Title, kr.Issue.URL))
				doc.WriteString(fmt.Sprintf("   - Issue: #%d (%s)\n",
					kr.Issue.Number, kr.Issue.URL))
				doc.WriteString(fmt.Sprintf("   - Status: %s\n", krIndicator.Status))

				// Add weekly updates section for KR - use rich formatting
				w.formatWeeklyUpdatesForKRGoogleDocsRich(&doc, kr, i+1, j+1)
				doc.WriteString("\n")
			}
		}

		doc.WriteString("---\n\n")
	}

	// Footer - match markdown format
	doc.WriteString("## 📝 Notes\n\n")
	doc.WriteString("- This report is automatically generated from GitHub issues and comments\n")
	doc.WriteString("- Status indicators are detected from weekly update comments\n")
	doc.WriteString("- Click on issue links to view full details and discussions\n")
	doc.WriteString(fmt.Sprintf("- Last updated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	return doc.String()
}

// StatusIndicator represents the visual status of an issue
type StatusIndicator struct {
	Status string
	Icon   string
	Color  string
}

func (w *Writer) getStatusIndicator(status entity.WeeklyUpdateStatus) StatusIndicator {
	switch status {
	case entity.StatusCompleted:
		return StatusIndicator{Status: "completed", Icon: "✅", Color: "green"}
	case entity.StatusBlocked:
		return StatusIndicator{Status: "blocked", Icon: "🚫", Color: "red"}
	case entity.StatusDelayed:
		return StatusIndicator{Status: "delayed", Icon: "🔴", Color: "red"}
	case entity.StatusCaution:
		return StatusIndicator{Status: "caution", Icon: "🟡", Color: "yellow"}
	case entity.StatusAtRisk:
		return StatusIndicator{Status: "at-risk", Icon: "⚠️", Color: "yellow"}
	case entity.StatusOnTrack:
		return StatusIndicator{Status: "on-track", Icon: "🟢", Color: "green"}
	default:
		return StatusIndicator{Status: "unknown", Icon: "❓", Color: "gray"}
	}
}

// googleDocsClient handles Google Docs API operations
type googleDocsClient struct {
	httpClient *http.Client
	ctx        context.Context
	writer     *Writer
}

// newGoogleDocsClientOAuth creates a new Google Docs client using OAuth2 user consent
func (w *Writer) newGoogleDocsClientOAuth(clientID, clientSecret string) (*googleDocsClient, error) {
	ctx := context.Background()

	// Find an available port for the callback server
	availablePort, err := w.findAvailablePort()
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %v", err)
	}

	// Create OAuth2 config with dynamic port
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  fmt.Sprintf("http://localhost:%d/callback", availablePort),
		Scopes:       []string{"https://www.googleapis.com/auth/documents"},
		Endpoint:     google.Endpoint,
	}

	// Get OAuth2 token through user consent flow
	token, err := w.getTokenFromWeb(config, availablePort)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth2 token: %v", err)
	}

	// Create HTTP client with token
	client := config.Client(ctx, token)

	return &googleDocsClient{
		httpClient: client,
		ctx:        ctx,
		writer:     w,
	}, nil
}

// findAvailablePort finds an available port for the OAuth callback server
func (w *Writer) findAvailablePort() (int, error) {
	ports := []int{8080, 8081, 8082, 8083, 8084}

	for _, port := range ports {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			listener.Close()
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports found in range 8080-8084")
}

// getTokenFromWeb uses OAuth2 user consent flow to get access token
func (w *Writer) getTokenFromWeb(config *oauth2.Config, port int) (*oauth2.Token, error) {
	// Generate auth URL
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	fmt.Printf("🔗 Opening browser for Google authentication...\n")

	// Try to open browser automatically
	if err := w.openBrowser(authURL); err != nil {
		fmt.Printf("⚠️ Could not open browser automatically: %v\n", err)
		fmt.Printf("Please manually go to:\n%v\n\n", authURL)
	} else {
		fmt.Printf("✅ Browser opened automatically\n")
		fmt.Printf("If browser doesn't open, manually go to:\n%v\n\n", authURL)
	}

	// Start local server to receive callback
	codeCh := make(chan string)
	errCh := make(chan error)

	go w.startCallbackServer(codeCh, errCh, port)

	// Wait for authorization code or error
	select {
	case code := <-codeCh:
		fmt.Printf("✅ Authorization successful!\n")

		// Exchange authorization code for token
		token, err := config.Exchange(context.Background(), code)
		if err != nil {
			return nil, fmt.Errorf("failed to exchange token: %v", err)
		}

		return token, nil

	case err := <-errCh:
		return nil, fmt.Errorf("authorization failed: %v", err)
	}
}

// startCallbackServer starts a local HTTP server to receive OAuth2 callback
func (w *Writer) startCallbackServer(codeCh chan<- string, errCh chan<- error, port int) {
	server := &http.Server{Addr: fmt.Sprintf(":%d", port)}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code received")
			return
		}

		// Send success response to browser
		fmt.Fprintf(w, `
			<html><body>
			<h2>✅ Authorization Successful!</h2>
			<p>You can close this browser tab and return to the terminal.</p>
			<script>window.close();</script>
			</body></html>
		`)

		codeCh <- code
		go server.Shutdown(context.Background())
	})

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		errCh <- err
	}
}

// openBrowser opens the default browser to the specified URL
func (w *Writer) openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// extractDocumentID extracts the document ID from a Google Docs URL
func (w *Writer) extractDocumentID(url string) string {
	// Google Docs URL format: https://docs.google.com/document/d/{DOCUMENT_ID}/edit
	re := regexp.MustCompile(`/document/d/([a-zA-Z0-9-_]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// createMarkdownFile creates a markdown file in the current directory with the content
func (w *Writer) createMarkdownFile(content string, projectInfo *entity.ProjectInfo) (string, error) {
	// Create a safe filename from project info
	filename := "okr-report"
	if projectInfo != nil && projectInfo.Owner != "" {
		if projectInfo.Repo != "" {
			filename = fmt.Sprintf("okr-report_%s_%s", projectInfo.Owner, projectInfo.Repo)
		} else {
			filename = fmt.Sprintf("okr-report_%s_project_%d", projectInfo.Owner, projectInfo.ProjectID)
		}
	}

	// Add timestamp to make it unique
	timestamp := time.Now().Format("20060102-150405")
	filename = fmt.Sprintf("%s_%s.md", filename, timestamp)

	// Write to current directory
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}
	
	filePath := filepath.Join(currentDir, filename)

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", err
	}

	return filePath, nil
}

// convertMarkdownToGoogleDocs converts markdown content to Google Docs format
func (gdc *googleDocsClient) convertMarkdownToGoogleDocs(documentURL, markdownContent string) error {
	documentID := gdc.writer.extractDocumentID(documentURL)
	if documentID == "" {
		return fmt.Errorf("invalid Google Docs URL: could not extract document ID")
	}

	fmt.Printf("🔗 Document ID: %s\n", documentID)

	// Create a new section with timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	sectionTitle := fmt.Sprintf("OKR Report - %s", timestamp)
	
	fmt.Printf("📑 Creating new section: %s\n", sectionTitle)
	
	// Convert markdown to plain text (comprehensive conversion)
	plainText := gdc.convertMarkdownToPlainText(markdownContent)
	fmt.Printf("📝 Converting %d chars of markdown to %d chars of plain text\n", len(markdownContent), len(plainText))

	// Add section header to the content
	contentWithHeader := fmt.Sprintf("=== %s ===\n\n%s\n\n", sectionTitle, plainText)
	
	if err := gdc.appendToDocument(documentID, contentWithHeader); err != nil {
		return fmt.Errorf("failed to append content to document: %v", err)
	}

	fmt.Printf("📤 Content appended successfully to document\n")

	// Skip formatting for now to avoid index issues
	fmt.Printf("⚠️ Skipping formatting to avoid API index errors\n")
	fmt.Printf("💡 Content inserted successfully without formatting\n")

	return nil
}

// convertMarkdownToPlainText performs comprehensive markdown to plain text conversion
func (gdc *googleDocsClient) convertMarkdownToPlainText(markdown string) string {
	text := markdown

	// Remove markdown syntax more aggressively to avoid index mismatches
	
	// Remove headers (# ## ### etc.) but keep the text
	text = regexp.MustCompile(`^#{1,6}\s+`).ReplaceAllStringFunc(text, func(match string) string {
		return "" // Remove the header markers completely
	})

	// Remove **bold** markers but keep the text
	text = regexp.MustCompile(`\*\*(.*?)\*\*`).ReplaceAllString(text, "$1")

	// Remove *italic* markers but keep the text (more careful to avoid conflicts)
	text = regexp.MustCompile(`(?:\*|_)([^*_\n]+)(?:\*|_)`).ReplaceAllString(text, "$1")

	// Remove `code` markers but keep the text
	text = regexp.MustCompile("`([^`\n]+)`").ReplaceAllString(text, "$1")

	// Remove ```code blocks``` but keep the content
	text = regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllStringFunc(text, func(match string) string {
		// Extract content between triple backticks
		content := strings.TrimPrefix(match, "```")
		content = strings.TrimSuffix(content, "```")
		// Remove language specifier from first line if present
		lines := strings.Split(content, "\n")
		if len(lines) > 1 && !strings.Contains(lines[0], " ") {
			content = strings.Join(lines[1:], "\n")
		}
		return content
	})

	// Remove [link text](url) but keep the link text
	text = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(text, "$1")

	// Remove horizontal rules
	text = regexp.MustCompile(`(?m)^[-*_]{3,}$`).ReplaceAllString(text, "")

	// Remove list markers (- * +) but keep the content
	text = regexp.MustCompile(`(?m)^(\s*)[-*+]\s+`).ReplaceAllString(text, "$1")

	// Clean up multiple consecutive newlines
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	return text
}

// buildBasicFormattingFromMarkdown creates basic formatting requests from the actual plain text content
func (gdc *googleDocsClient) buildBasicFormattingFromMarkdown(markdownContent string) []map[string]interface{} {
	// Disable formatting for now to avoid index issues - just return empty requests
	// This ensures the content gets inserted without formatting errors
	return []map[string]interface{}{}
}

// writeToGoogleDocs writes rich formatted content to a Google Docs document
func (gdc *googleDocsClient) writeToGoogleDocs(documentURL string, objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, analysis string) error {
	documentID := gdc.writer.extractDocumentID(documentURL)
	if documentID == "" {
		return fmt.Errorf("invalid Google Docs URL: could not extract document ID")
	}

	fmt.Printf("📝 Writing to Google Docs document: %s\n", documentID)

	// First, clear the document content
	if err := gdc.clearDocument(documentID); err != nil {
		return fmt.Errorf("failed to clear document: %v", err)
	}

	// Then insert the rich formatted content
	if err := gdc.insertRichContent(documentID, objectives, projectInfo, analysis); err != nil {
		return fmt.Errorf("failed to insert rich content: %v", err)
	}

	fmt.Printf("✅ Successfully updated Google Docs document with rich formatting\n")
	return nil
}

// clearDocument removes all content from the document
func (gdc *googleDocsClient) clearDocument(documentID string) error {
	// Get document to find the end index
	doc, err := gdc.getDocument(documentID)
	if err != nil {
		return err
	}

	// Extract end index from document
	body, ok := doc["body"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid document structure: missing body")
	}

	content, ok := body["content"].([]interface{})
	if !ok || len(content) == 0 {
		return nil // Document is already empty
	}

	// Calculate the total content length
	var totalLength int
	for _, element := range content {
		if elem, ok := element.(map[string]interface{}); ok {
			if endIdx, exists := elem["endIndex"]; exists {
				if idx, ok := endIdx.(float64); ok {
					if int(idx) > totalLength {
						totalLength = int(idx)
					}
				}
			}
		}
	}

	// Google Docs always has at least one character (the final newline)
	// Only attempt to delete if there's content beyond the mandatory newline
	if totalLength <= 1 {
		return nil // Document only contains the mandatory newline, nothing to delete
	}

	// Calculate safe range - leave at least one character to avoid empty range
	startIndex := 1
	endIndex := totalLength - 1

	// Ensure we don't create an empty range
	if endIndex <= startIndex {
		return nil // Range would be empty, skip deletion
	}

	// Create delete request
	requests := []map[string]interface{}{
		{
			"deleteContentRange": map[string]interface{}{
				"range": map[string]interface{}{
					"startIndex": startIndex,
					"endIndex":   endIndex,
				},
			},
		},
	}

	return gdc.batchUpdate(documentID, requests)
}

// insertRichContent inserts rich formatted content into the document
func (gdc *googleDocsClient) insertRichContent(documentID string, objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, analysis string) error {
	// First, build the plain text content
	content := gdc.buildPlainTextContent(objectives, projectInfo, analysis)

	// Insert all text at once
	insertRequests := []map[string]interface{}{
		{
			"insertText": map[string]interface{}{
				"location": map[string]interface{}{
					"index": 1,
				},
				"text": content,
			},
		},
	}

	// Apply text insertion first
	if err := gdc.batchUpdate(documentID, insertRequests); err != nil {
		return fmt.Errorf("failed to insert text: %v", err)
	}

	// Now apply formatting in a second batch
	formattingRequests := gdc.buildFormattingRequests(objectives, projectInfo, analysis)
	if len(formattingRequests) > 0 {
		if err := gdc.batchUpdate(documentID, formattingRequests); err != nil {
			return fmt.Errorf("failed to apply formatting: %v", err)
		}
	}

	return nil
}

// buildPlainTextContent builds the complete plain text content for the document
func (gdc *googleDocsClient) buildPlainTextContent(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, analysis string) string {
	var content strings.Builder

	// Title
	title := "OKR Report"
	if gdc.writer != nil && gdc.writer.config != nil && gdc.writer.config.Output.Title != "" {
		title = gdc.writer.config.Output.Title
	}
	content.WriteString(title + "\n\n")

	// Project info
	projectName := "Project"
	if gdc.writer != nil && gdc.writer.config != nil && gdc.writer.config.Output.ProjectName != "" {
		projectName = gdc.writer.config.Output.ProjectName
	}
	projectUrl := fmt.Sprintf("https://github.com/orgs/%s/projects/%d/views/%d",
		projectInfo.Owner, projectInfo.ProjectID, projectInfo.ViewID)
	content.WriteString(fmt.Sprintf("📊 Project: %s (%s)\n\n", projectName, projectUrl))

	// Generated timestamp
	content.WriteString(fmt.Sprintf("📅 Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// AI Analysis Section (if available)
	if analysis != "" {
		content.WriteString("## 🤖 AI Analysis\n\n")
		content.WriteString(analysis)
		content.WriteString("\n\n")
		content.WriteString("---\n\n")
	}

	// Summary section
	content.WriteString("## 📈 Summary\n\n")

	// Calculate summary stats
	totalObjectives := len(objectives)
	totalKRs := 0
	completedKRs := 0
	blockedKRs := 0
	delayedKRs := 0
	cautionKRs := 0
	atRiskKRs := 0
	onTrackKRs := 0

	for _, obj := range objectives {
		totalKRs += len(obj.ChildIssues)
		for _, kr := range obj.ChildIssues {
			switch kr.GetKRStatus() {
			case entity.StatusCompleted:
				completedKRs++
			case entity.StatusBlocked:
				blockedKRs++
			case entity.StatusDelayed:
				delayedKRs++
			case entity.StatusCaution:
				cautionKRs++
			case entity.StatusAtRisk:
				atRiskKRs++
			case entity.StatusOnTrack:
				onTrackKRs++
			}
		}
	}

	// Summary bullets (match Markdown style with dashes)
	content.WriteString(fmt.Sprintf("- Objectives: %d\n", totalObjectives))
	content.WriteString(fmt.Sprintf("- Key Results: %d\n", totalKRs))
	content.WriteString(fmt.Sprintf("- ✅ Completed: %d\n", completedKRs))
	content.WriteString(fmt.Sprintf("- 🟢 On Track: %d\n", onTrackKRs))
	content.WriteString(fmt.Sprintf("- 🟡 Caution: %d\n", cautionKRs))
	content.WriteString(fmt.Sprintf("- ⚠️ At Risk: %d\n", atRiskKRs))
	content.WriteString(fmt.Sprintf("- 🔴 Delayed: %d\n", delayedKRs))
	content.WriteString(fmt.Sprintf("- 🚫 Blocked: %d\n\n", blockedKRs))

	// Progress bar (match Markdown format exactly)
	if totalKRs > 0 {
		completionRate := float64(completedKRs) / float64(totalKRs) * 100
		content.WriteString(fmt.Sprintf("Overall Progress: %.1f%% (%d/%d completed)\n\n", completionRate, completedKRs, totalKRs))

		// Visual progress bar (match Markdown style)
		progressBars := int(completionRate / 10)
		content.WriteString("```\n")
		content.WriteString("Progress: [")
		for i := 0; i < 10; i++ {
			if i < progressBars {
				content.WriteString("█")
			} else {
				content.WriteString("░")
			}
		}
		content.WriteString(fmt.Sprintf("] %.1f%%\n", completionRate))
		content.WriteString("```\n\n")
	}

	content.WriteString("---\n\n")

	// Objectives and KRs section (match Markdown ## style)
	content.WriteString("## 🎯 Objectives & Key Results\n\n")

	// Process each objective (match Markdown format exactly)
	for i, obj := range objectives {
		objStatus := obj.GetObjectiveStatus()
		indicator := gdc.writer.getStatusIndicator(objStatus)

		// Objective heading (match Markdown ### style)
		content.WriteString(fmt.Sprintf("### %d. %s %s\n", i+1, indicator.Icon, obj.Issue.Title))
		content.WriteString(fmt.Sprintf("**Issue**: [#%d](%s) | **Status**: %s\n\n", obj.Issue.Number, obj.Issue.URL, indicator.Status))

		// Key Results (match Markdown #### style)
		if len(obj.ChildIssues) > 0 {
			content.WriteString("#### 📋 Key Results:\n\n")

			for j, kr := range obj.ChildIssues {
				krStatus := kr.GetKRStatus()
				krIndicator := gdc.writer.getStatusIndicator(krStatus)

				// KR title with status (match Markdown format)
				content.WriteString(fmt.Sprintf("%d.%d. %s **[%s](%s)**\n", i+1, j+1, krIndicator.Icon, kr.Issue.Title, kr.Issue.URL))
				content.WriteString(fmt.Sprintf("   - **Issue**: [#%d](%s)\n", kr.Issue.Number, kr.Issue.URL))
				content.WriteString(fmt.Sprintf("   - **Status**: %s\n", krIndicator.Status))

				// Weekly updates for KRs (match Markdown format)
				weeklyUpdates := gdc.writer.getWeeklyUpdates(kr.AllUpdates)
				if len(weeklyUpdates) > 0 {
					content.WriteString("   - **Weekly Updates**:\n")

					maxUpdates := 2
					if len(weeklyUpdates) < maxUpdates {
						maxUpdates = len(weeklyUpdates)
					}

					for k := 0; k < maxUpdates; k++ {
						update := weeklyUpdates[k]
						updateLabel := "Latest"
						if k == 1 {
							updateLabel = "Previous"
						}

						content.WriteString(fmt.Sprintf("     - **%s** (%s by @%s):\n", updateLabel, update.Date, update.Author))

						// Format the update content nicely (preserve Markdown structure)
						formattedContent := gdc.formatUpdateContentForGoogleDocs(update.Content)
						content.WriteString(formattedContent + "\n")
					}
				}
				content.WriteString("\n")
			}
		}
		content.WriteString("---\n\n")
	}

	// Footer (match Markdown ## style)
	content.WriteString("## 📝 Notes\n\n")
	content.WriteString("- This report is automatically generated from GitHub issues and comments\n")
	content.WriteString("- Status indicators are detected from weekly update comments\n")
	content.WriteString("- Click on issue links to view full details and discussions\n")
	content.WriteString(fmt.Sprintf("- Last updated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	return content.String()
}

// buildFormattingRequests builds the formatting requests to apply basic styling safely
func (gdc *googleDocsClient) buildFormattingRequests(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, analysis string) []map[string]interface{} {
	// Use a simplified approach to avoid complex index tracking that can cause errors
	// Focus on major headings that are easy to find and format

	var requests []map[string]interface{}

	// Get the complete content to analyze
	content := gdc.buildPlainTextContent(objectives, projectInfo, analysis)

	// Find and format the main title (first line)
	title := "OKR Report"
	if gdc.writer != nil && gdc.writer.config != nil && gdc.writer.config.Output.Title != "" {
		title = gdc.writer.config.Output.Title
	}

	titleEnd := 1 + len(title)
	requests = append(requests, map[string]interface{}{
		"updateParagraphStyle": map[string]interface{}{
			"range": map[string]interface{}{
				"startIndex": 1,
				"endIndex":   titleEnd,
			},
			"paragraphStyle": map[string]interface{}{
				"namedStyleType": "TITLE",
			},
			"fields": "namedStyleType",
		},
	})

	// Find and format major headings using string search
	headings := []struct {
		text      string
		styleType string
	}{
		{"## 🤖 AI Analysis", "HEADING_1"},
		{"## 📈 Summary", "HEADING_1"},
		{"## 🎯 Objectives & Key Results", "HEADING_1"},
		{"## 📝 Notes", "HEADING_1"},
	}

	for _, heading := range headings {
		startIndex := strings.Index(content, heading.text)
		if startIndex >= 0 {
			// Adjust for document position (content starts at index 1)
			docStartIndex := startIndex + 1
			docEndIndex := docStartIndex + len(heading.text)

			requests = append(requests, map[string]interface{}{
				"updateParagraphStyle": map[string]interface{}{
					"range": map[string]interface{}{
						"startIndex": docStartIndex,
						"endIndex":   docEndIndex,
					},
					"paragraphStyle": map[string]interface{}{
						"namedStyleType": heading.styleType,
					},
					"fields": "namedStyleType",
				},
			})
		}
	}

	// Find and format objective headings (### style)
	for i := range objectives {
		objHeadingPrefix := fmt.Sprintf("### %d. ", i+1)
		startIndex := strings.Index(content, objHeadingPrefix)
		if startIndex >= 0 {
			// Find the end of the line
			lineEnd := strings.Index(content[startIndex:], "\n")
			if lineEnd >= 0 {
				docStartIndex := startIndex + 1
				docEndIndex := docStartIndex + lineEnd

				requests = append(requests, map[string]interface{}{
					"updateParagraphStyle": map[string]interface{}{
						"range": map[string]interface{}{
							"startIndex": docStartIndex,
							"endIndex":   docEndIndex,
						},
						"paragraphStyle": map[string]interface{}{
							"namedStyleType": "HEADING_2",
						},
						"fields": "namedStyleType",
					},
				})
			}
		}
	}

	// Find and format KR headings (#### style)
	krHeadingText := "#### 📋 Key Results:"
	startIndex := 0
	for {
		startIndex = strings.Index(content[startIndex:], krHeadingText)
		if startIndex == -1 {
			break
		}

		docStartIndex := startIndex + 1
		docEndIndex := docStartIndex + len(krHeadingText)

		requests = append(requests, map[string]interface{}{
			"updateParagraphStyle": map[string]interface{}{
				"range": map[string]interface{}{
					"startIndex": docStartIndex,
					"endIndex":   docEndIndex,
				},
				"paragraphStyle": map[string]interface{}{
					"namedStyleType": "HEADING_3",
				},
				"fields": "namedStyleType",
			},
		})

		startIndex += len(krHeadingText)
	}

	return requests
}

// formatUpdateContentForGoogleDocs formats weekly update content for Google Docs
func (gdc *googleDocsClient) formatUpdateContentForGoogleDocs(content string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip weekly update headers
		if strings.HasPrefix(strings.ToLower(trimmedLine), "# weekly update") {
			continue
		}

		// Clean HTML tags from the line
		cleanedLine := gdc.cleanHTMLTags(trimmedLine)

		// Add indentation for content
		if cleanedLine != "" {
			result.WriteString("       " + cleanedLine + "\n")
		} else {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// cleanHTMLTags removes HTML tags and cleans up content for Google Docs
func (gdc *googleDocsClient) cleanHTMLTags(text string) string {
	// Remove HTML tags using regex
	re := regexp.MustCompile(`<[^>]*>`)
	cleaned := re.ReplaceAllString(text, "")

	// Decode common HTML entities
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", " ")
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	cleaned = strings.ReplaceAll(cleaned, "&quot;", "\"")
	cleaned = strings.ReplaceAll(cleaned, "&#39;", "'")

	// Clean up extra whitespace
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}

// getDocument retrieves document information
func (gdc *googleDocsClient) getDocument(documentID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://docs.googleapis.com/v1/documents/%s", documentID)

	resp, err := gdc.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google Docs API error: %d - %s", resp.StatusCode, string(body))
	}

	var doc map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}

	return doc, nil
}

// appendToDocument appends content to the end of a Google Docs document
func (gdc *googleDocsClient) appendToDocument(documentID, content string) error {
	// First, get the current document to find the end
	doc, err := gdc.getDocument(documentID)
	if err != nil {
		return fmt.Errorf("failed to get document: %v", err)
	}

	// Extract end index from document
	body, ok := doc["body"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid document structure: missing body")
	}

	docContent, ok := body["content"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid document structure: missing content")
	}

	// Calculate the document end index
	var endIndex int = 1
	for _, element := range docContent {
		if elem, ok := element.(map[string]interface{}); ok {
			if idx, exists := elem["endIndex"]; exists {
				if i, ok := idx.(float64); ok {
					if int(i) > endIndex {
						endIndex = int(i)
					}
				}
			}
		}
	}

	// Insert page break (if document has content) and new content
	var requests []map[string]interface{}
	
	// Only add page break if document has substantial content (more than just the default newline)
	if endIndex > 2 {
		requests = append(requests, map[string]interface{}{
			"insertPageBreak": map[string]interface{}{
				"location": map[string]interface{}{
					"index": endIndex - 1,
				},
			},
		})
		// Adjust insert location after page break
		endIndex++
	}
	
	// Insert the new content
	requests = append(requests, map[string]interface{}{
		"insertText": map[string]interface{}{
			"location": map[string]interface{}{
				"index": endIndex - 1,
			},
			"text": content,
		},
	})

	return gdc.batchUpdate(documentID, requests)
}

// batchUpdate performs batch updates to the document
func (gdc *googleDocsClient) batchUpdate(documentID string, requests []map[string]interface{}) error {
	url := fmt.Sprintf("https://docs.googleapis.com/v1/documents/%s:batchUpdate", documentID)

	payload := map[string]interface{}{
		"requests": requests,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := gdc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Google Docs API error: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// ReportGenerator implements the ReportGenerator interface
type ReportGenerator struct {
	writer *Writer
}

// NewReportGenerator creates a new report generator
func NewReportGenerator() *ReportGenerator {
	return &ReportGenerator{
		writer: NewWriter(),
	}
}

// NewReportGeneratorWithConfig creates a new report generator with configuration
func NewReportGeneratorWithConfig(config *entity.Config) *ReportGenerator {
	return &ReportGenerator{
		writer: NewWriterWithConfig(config),
	}
}

// GenerateReport generates a report in the specified format
func (r *ReportGenerator) GenerateReport(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, format ports.OutputFormat, filename string) error {
	switch format {
	case ports.OutputFormatMarkdown:
		return r.writer.WriteMarkdown(objectives, projectInfo, filename)
	case ports.OutputFormatJSON:
		return r.writer.WriteJSON(objectives, filename)
	case ports.OutputFormatGoogleDocs:
		// For Google Docs, just create a plain text file as fallback
		content := r.writer.formatAsGoogleDocs(objectives, projectInfo)
		return os.WriteFile(filename, []byte(content), 0644)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// GenerateReportWithGoogleDocs generates a report with Google Docs integration
func (r *ReportGenerator) GenerateReportWithGoogleDocs(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, format ports.OutputFormat, filename, documentURL, clientID, clientSecret string) error {
	if format == ports.OutputFormatGoogleDocs && documentURL != "" && clientID != "" && clientSecret != "" {
		return r.writer.WriteGoogleDocs(objectives, projectInfo, documentURL, clientID, clientSecret)
	}
	// Fallback to regular report generation
	return r.GenerateReport(objectives, projectInfo, format, filename)
}

// GenerateReportWithGoogleDocsAndAnalysis generates a report with Google Docs integration and AI analysis
func (r *ReportGenerator) GenerateReportWithGoogleDocsAndAnalysis(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo, format ports.OutputFormat, filename, documentURL, clientID, clientSecret, analysis string) error {
	if format == ports.OutputFormatGoogleDocs && documentURL != "" && clientID != "" && clientSecret != "" {
		return r.writer.WriteGoogleDocsWithAnalysis(objectives, projectInfo, documentURL, clientID, clientSecret, analysis)
	}
	// Fallback to regular report generation
	return r.GenerateReport(objectives, projectInfo, format, filename)
}

// FormatAsMarkdown returns markdown formatted content
func (r *ReportGenerator) FormatAsMarkdown(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo) string {
	return r.writer.formatAsMarkdown(objectives, projectInfo)
}

// FormatAsJSON returns JSON formatted content
func (r *ReportGenerator) FormatAsJSON(objectives []*entity.IssueWithUpdates) (string, error) {
	data, err := json.MarshalIndent(objectives, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshaling JSON: %v", err)
	}
	return string(data), nil
}

// FormatAsGoogleDocs returns Google Docs compatible plain text content
func (r *ReportGenerator) FormatAsGoogleDocs(objectives []*entity.IssueWithUpdates, projectInfo *entity.ProjectInfo) string {
	return r.writer.formatAsGoogleDocs(objectives, projectInfo)
}
