# to memorize

# GitHub OKR Fetcher

This project contains a Go application that fetches GitHub issues and their weekly update comments from GitHub project views, implementing a complete OKR (Objectives and Key Results) tracking system.

## Features

- **Configuration-driven**: JSON config file support for tokens, URLs, and labels
- **Smart filtering**: AND condition for multiple required labels
- **Search-based queries**: GitHub search API integration for efficient label filtering
- **Parent-child relationships**: Uses explicit "Parent Issue:" references for OKR hierarchy
- **Weekly updates**: Extracts and displays latest "weekly update yyyy-mm-dd" comments
- **Progress tracking**: Visual progress bars and completion metrics
- **Multiple output formats**: Markdown reports, JSON data export, and Google Docs integration
- **Rate limiting**: Built-in GitHub API rate limiting and retry mechanisms
- **Caching**: In-memory API response caching for performance
- **Clean Architecture**: Hexagonal architecture with ports and adapters pattern

## Usage

### Configuration File Approach (Recommended)

```bash
# Generate example config
go run main.go -generate-config

# Edit config.json with your details, then run
go run main.go
```

### Command Line Approach

```bash
# Generate markdown report (default)
go run main.go -url="https://github.com/orgs/your-org/projects/123/views/456"

# With custom labels
go run main.go -url="..." -labels="kind/okr,team/network,target/2026-q1"

# Generate JSON output
go run main.go -url="..." -json

# Skip label filtering
go run main.go -url="..." -skip-labels

# Build from source
go build -o github-okr-fetcher
```

## Configuration

### Example config.json:
```json
{
  "github": {
    "token": "your_github_token_here",
    "url": "https://github.com/orgs/your-org/projects/123/views/456"
  },
  "labels": {
    "required": ["kind/okr", "team/network", "target/2026-q1"]
  },
  "filter": {
    "query": "label:\"target/2026-Q1\" label:\"kind/okr\" is:issue",
    "use_search": true
  },
  "output": {
    "format": "markdown",
    "file": ""
  }
}
```

## Requirements

- Go 1.19 or later
- GitHub personal access token (set as GITHUB_TOKEN environment variable or in config file)
- Access to the GitHub repository/project

## OKR Structure

### Issue Classification:
- **Objectives**: Issues with no "Parent Issue:" reference (top-level issues)
- **Key Results**: Issues with explicit "Parent Issue:" references in title or body

### Hierarchy Detection:
The application uses a simple and reliable approach:
1. **Search-based filtering**: First filters issues using GitHub search API with configured labels and query
2. **Parent Issue references**: Looks for explicit "Parent Issue: #123" or "Parent Issue: https://github.com/..." in issue title or body
3. **Automatic classification**: Issues with parent references become Key Results, issues without become Objectives

### Example Structure:
```
Objective: Drive Infrastructure Modernization (#25497)
├── KR1: Foundation/Backend team enabled for ImageFlux migration (#25498)
│   └── Parent Issue: https://github.com/.../issues/25497
├── KR2: Deployment capability for gateways via WSCD (#25499)
│   └── Parent Issue: https://github.com/.../issues/25497
└── KR3: Standardize Istio ingress gateway usage (#25519)
    └── Parent Issue: https://github.com/.../issues/25497
```

## Environment Variables

- `GITHUB_TOKEN`: Personal access token for GitHub API authentication

## Output

**Default Markdown Report** (`okr-report_orgname_project_view.md`):
- 📊 Executive summary with progress tracking
- 🎯 Visual status indicators (✅🟢⚠️🚫)
- 📋 Structured objectives and key results
- 🔗 Clickable GitHub issue links
- 📈 Progress bars and completion metrics
- 📝 Latest weekly update summaries

**JSON Output** (with `-json` flag):
- Raw structured data for integration with other tools
- Complete issue details and weekly update history

## Recent Updates

### Google Docs Rich API Formatting with Native Styles (Latest)
- **Problem**: Google Docs output was plain text without proper formatting, native links, or professional styling
- **Solution**: Implemented native Google Docs API formatting with `insertRichContent()` method using proper API calls
- **Features**: 
  - **Native Google Docs styling**: Uses proper TITLE, HEADING_1, HEADING_2, HEADING_3 styles
  - **Clickable hyperlinks**: Real hyperlinks using Google Docs link API (not text URLs)
  - **Rich text formatting**: Bold text, italic text, colored links, and monospace fonts
  - **Professional structure**: Proper headings, bullet points, and indentation
  - **Visual progress bars**: Monospace font for proper character alignment
  - **Emoji preservation**: All status emojis and visual indicators maintained
  - **Structured weekly updates**: Properly formatted with bold headers and indented content
- **Result**: Google Docs now displays native rich formatting with proper styles, clickable links, and professional appearance

### KR Status Based on Latest Weekly Update Symbol
- **Problem**: KRs showed ❓ (unknown status) even when they had weekly update comments with status symbols
- **Solution**: Implemented `GetKRStatus()` method that prioritizes status symbols from the most recent weekly update
- **Logic**: 
  - Searches through all weekly updates to find the latest one with a valid status symbol
  - Prioritizes detected status symbols (🟢🟡🔴⚠️🚫✅) from weekly update content
  - Falls back to `LatestUpdate.Status` if no weekly updates have symbols
  - Applies same business rules (closed issues = completed, "completed" + open = on-track)
- **Result**: KRs now show status indicators based on their weekly update symbols instead of ❓

### Objective Status Based on KR Aggregation
- **Problem**: Objectives showed ❓ (unknown status) when they had valid KR statuses
- **Solution**: Implemented `GetObjectiveStatus()` method that derives objective status from child KR statuses
- **Logic**: 
  - Blocked/Delayed/AtRisk/Caution KRs override objective status (highest priority)
  - All KRs completed = Objective completed
  - ≥50% KRs completed = Objective on-track
  - Any on-track KRs = Objective on-track
  - Default to unknown only if all KRs are unknown
- **Result**: Objectives now show meaningful status indicators based on their KR progress instead of ❓

### Simplified Parent-Child Relationship Detection
- **Problem**: Complex pattern matching and API calls made relationship detection unreliable
- **Solution**: Simplified to only look for explicit "Parent Issue:" references in title/body
- **Result**: Clean, reliable parent-child relationships based on explicit references

### Search-Based Filtering (Fixed)
- **Problem**: Application was fetching all project issues instead of using search query
- **Solution**: Implemented proper GitHub search API integration with label filtering
- **Result**: Efficient filtering - fetches only relevant issues (13 vs 1,196 total issues)

### KR Duplication Issue (Fixed)
- **Problem**: All KRs were being assigned to every objective, creating duplicates
- **Solution**: Fixed assignment logic to properly distribute KRs to objectives
- **Result**: Clean 1:1 or 1:many relationships between objectives and KRs