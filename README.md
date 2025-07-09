# GitHub OKR Fetcher

A powerful Go-based tool for fetching and tracking OKRs (Objectives and Key Results) from GitHub Projects. It automatically generates progress reports by analyzing GitHub issues and their weekly update comments, with intelligent status detection and rich formatting capabilities.

## 🌟 Features

### 📊 **Smart Status Detection**
- **Intelligent KR Status**: Prioritizes latest weekly update symbols (🟢🟡🔴⚠️🚫✅) over generic status
- **Objective Aggregation**: Automatically derives objective status from child Key Results
- **Visual Status Indicators**: Clear, color-coded status indicators throughout reports
- **Weekly Update Parsing**: Extracts status from "weekly update YYYY-MM-DD" comment patterns

### 📝 **Rich Output Formats**
- **Professional Markdown**: Rich formatting with emojis, progress bars, and clickable links
- **Native Google Docs**: Rich API formatting with proper headings, hyperlinks, and styling
- **Structured JSON**: Complete data export for integration and automation
- **AI-Enhanced Reports**: Optional LiteLLM integration for insights and business impact analysis

### 🔍 **Advanced Filtering & Search**
- **Configuration-driven**: Flexible JSON config file support for team customization
- **Smart label filtering**: Advanced AND conditions with multiple required labels
- **Search-based queries**: Efficient GitHub search API integration for large repositories
- **Parent-child relationships**: Automatic OKR hierarchy detection via explicit references

### ⚡ **Performance & Reliability**
- **Caching system**: In-memory response caching for improved performance
- **Rate limiting**: Built-in GitHub API rate limiting with retry mechanisms
- **Concurrent processing**: Optimized parallel API calls
- **Error handling**: Comprehensive error recovery and fallback mechanisms

### 🛠️ **Professional Tooling**
- **Cobra CLI**: Professional command-line interface with subcommands and help
- **Hexagonal architecture**: Clean, maintainable, and testable code structure
- **Environment-based security**: Secure token management via environment variables
- **Timestamped outputs**: Automatic timestamping and version tracking

## 📋 Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Usage](#usage)
- [Output Formats](#output-formats)
- [AI Analysis](#ai-analysis)
- [Architecture](#architecture)
  - [Architecture Diagram](docs/architecture.md)
- [OKR Structure](#okr-structure)
- [Development](#development)
- [Contributing](#contributing)

## 🚀 Installation

### Prerequisites

- Go 1.23 or later
- GitHub personal access token with repository access
- Access to the target GitHub project

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/github-okr-fetcher.git
cd github-okr-fetcher

# Install dependencies
go mod download

# Build the binary
go build -o github-okr-fetcher

# Or run directly
go run main.go
```

## 🏃 Quick Start

### 1. Generate Configuration File

```bash
# Using built binary
./github-okr-fetcher generate-config

# Or run directly from source
go run main.go generate-config
```

This creates a `config.json` file with example settings.

### 2. Set Up Your GitHub Token

```bash
export GITHUB_TOKEN="your_github_personal_access_token"
```

### 3. Edit Configuration

Update `config.json` with your project details:

```json
{
  "github": {
    "url": "https://github.com/orgs/your-org/projects/123/views/456"
  },
  "labels": {
    "required": ["kind/okr", "team/your-team", "target/2026-q1"]
  },
  "output": {
    "format": "markdown"
  }
}
```

### 4. Run the Tool

```bash
# Using built binary
./github-okr-fetcher

# Or run directly from source
go run main.go
```

## ⚙️ Configuration

### Configuration File Structure

```json
{
  "github": {
    "url": "https://github.com/orgs/your-org/projects/123/views/456",
    "owner": "your-org",                      // Optional: extracted from URL
    "repo": "your-repo",                      // Optional: defaults to "microservices"
    "timeout_seconds": 30,                   // HTTP timeout
    "rate_limit_per_hour": 5000,            // GitHub API rate limit
    "max_retries": 3,                        // Retry attempts
    "page_size": 100,                        // API page size
    "max_issues_limit": 10000,              // Memory protection limit
    "user_agent": "GitHub-OKR-Fetcher/1.0"  // HTTP User Agent
  },
  "labels": {
    "required": [                             // AND condition for all labels
      "kind/okr",
      "team/network",
      "target/2026-q1"
    ]
  },
  "filter": {
    "query": "label:\"target/2026-Q1\" label:\"kind/okr\" is:issue",
    "use_search": true                        // Use GitHub search API
  },
  "output": {
    "format": "markdown",                     // Options: markdown, json, google-docs
    "file": "custom-output.md",              // Optional: custom output filename
    "title": "Your OKR Report Title",        // Report title
    "filename_pattern": "okr-report_%s_%d_%d_%s%s", // File naming pattern
    "timestamp_format": "20060102_150405",   // Timestamp format
    "progress_bar_segments": 10,             // Progress bar segments
    "google_docs": {                         // Google Docs integration settings
      "url": "https://docs.google.com/document/d/YOUR_DOC_ID/edit"
      // OAuth credentials now use environment variables for security
    }
  },
  "litellm": {                               // AI analysis configuration
    "enabled": false,                        // Enable AI-powered analysis
    "base_url": "https://api.openai.com",    // LiteLLM API endpoint
    "model": "gpt-4o",                       // AI model to use
    "timeout_seconds": 60,                   // Request timeout
    "analysis_word_limit": 100               // Word limit for analysis
    // API token now uses LITELLM_TOKEN environment variable for security
  },
  "performance": {
    "max_concurrency": 10,                   // Max parallel API calls
    "rate_limit_per_hour": 5000,            // GitHub API rate limit
    "cache_enabled": true                    // Enable response caching
  },
  "default_values": {
    "organization": "your-org",
    "repository": "your-repo"
  }
}
```

### Environment Variables

**Required:**
- `GITHUB_TOKEN`: GitHub personal access token (required for API access)

**Optional:**
- `GOOGLE_CLIENT_ID`: Google OAuth2 Client ID (required for Google Docs integration)
- `GOOGLE_CLIENT_SECRET`: Google OAuth2 Client Secret (required for Google Docs integration)  
- `LITELLM_TOKEN`: LiteLLM API token (required for AI analysis features)

**Setup:**
```bash
# Copy the example environment file
cp .env.example .env

# Edit .env with your actual credentials
# NEVER commit .env file to version control
```

## 📖 Usage

### Command Line Options

```bash
# View help
./github-okr-fetcher --help

# Basic usage with config file
./github-okr-fetcher

# Generate configuration file
./github-okr-fetcher generate-config

# Override project URL
./github-okr-fetcher --url="https://github.com/orgs/your-org/projects/123/views/456"

# Custom labels (overrides config)
./github-okr-fetcher --labels="kind/okr,team/platform,target/2026-q1"

# Generate JSON output
./github-okr-fetcher --json

# Skip label filtering
./github-okr-fetcher --skip-labels

# Use custom config file
./github-okr-fetcher --config="custom-config.json"

# Specify output file
./github-okr-fetcher --output="my-okr-report.md"
```

### Flag Reference

| Flag | Short | Description |
|------|-------|-------------|
| `--url` | `-u` | GitHub project view URL (overrides config) |
| `--output` | `-o` | Output file path (overrides config) |
| `--json` | `-j` | Output JSON instead of Markdown |
| `--labels` | `-l` | Comma-separated list of required labels |
| `--config` | `-c` | Config file path (default: config.json) |
| `--google-docs` | | Output Google Docs compatible format |
| `--skip-labels` | | Skip label filtering and process all issues |
| `--help` | `-h` | Show help information |

### Examples

#### Generate OKR Report for Specific Quarter

```bash
./github-okr-fetcher \
  --url="https://github.com/orgs/myorg/projects/10/views/1" \
  --labels="kind/okr,target/2026-q1"
```

#### Export to JSON for Further Processing

```bash
./github-okr-fetcher --json --output="okr-data.json"
```

#### Generate Google Docs Compatible Report

```bash
./github-okr-fetcher --google-docs
```

#### Using Source Code Directly

```bash
# Basic usage
go run main.go

# With flags
go run main.go --url="..." --json
```

## 📊 Output Formats

### 1. Markdown Report (Default)

Generates a comprehensive markdown report with:
- 📈 Executive summary with progress metrics
- 🎯 Visual status indicators (✅ 🟢 ⚠️ 🚫)
- 📋 Hierarchical OKR structure
- 🔗 Direct links to GitHub issues
- 📊 Progress bars and completion rates
- 💬 Latest weekly update summaries

Example output: `okr-report_orgname_123_456_20250709_143052.md`

**Note**: All output files include timestamps in the format `YYYYMMDD_HHMMSS` for easy tracking and version management.

### 2. JSON Export

Structured data export for integration with other tools:
```json
[
  {
    "issue": {
      "number": 25497,
      "title": "Drive Infrastructure Modernization",
      "url": "https://github.com/...",
      "type": "objective"
    },
    "latest_update": {
      "date": "2025-07-04",
      "content": "Weekly update content...",
      "author": "username",
      "status": "on-track"
    },
    "child_issues": [...]
  }
]
```

### 3. Google Docs Integration (Rich Native Formatting)

Direct export to Google Docs with professional native formatting:

#### **Rich API Formatting Features**
- **Native Google Docs Styles**: Proper TITLE, HEADING_1, HEADING_2, HEADING_3 styles
- **Clickable Hyperlinks**: Real hyperlinks using Google Docs link API (not plain text URLs)
- **Text Formatting**: Bold, italic, colored text, and monospace fonts for progress bars
- **Professional Structure**: Proper headings, bullet points, and indented content
- **Visual Progress Bars**: Monospace font ensures proper character alignment ([█████░░░░░])
- **Status Emojis**: All status indicators and visual elements preserved (📊📅🎯🟢🔴⚠️🚫✅)
- **Structured Weekly Updates**: Rich formatting with bold headers and organized sections

#### **OAuth2 Authentication**
```bash
# Set up Google OAuth credentials
export GOOGLE_CLIENT_ID="your_google_client_id"
export GOOGLE_CLIENT_SECRET="your_google_client_secret"

# Configure document URL in config.json
{
  "output": {
    "format": "google-docs",
    "google_docs": {
      "url": "https://docs.google.com/document/d/YOUR_DOCUMENT_ID/edit"
    }
  }
}
```

#### **Professional Output**
The Google Docs output provides:
- Document title in proper TITLE style
- Hierarchical section headings
- Clickable links to GitHub issues and projects
- Rich weekly update formatting with structured content
- Visual progress tracking with proper alignment
- Professional appearance matching enterprise document standards

## 🤖 AI Analysis (Optional)

The tool includes optional LiteLLM integration for AI-powered OKR analysis and business impact insights:

### Features
- **Success & Achievements**: Automatically identifies completed issues and key milestones
- **Business Impact**: Provides quantitative and qualitative impact metrics
- **Concise Summaries**: Focused 100-word analysis for executive-level insights
- **Multiple AI Models**: Supports OpenAI, Anthropic, and other LiteLLM-compatible providers
- **Security-First**: API tokens managed via environment variables

### Configuration

**Environment Setup:**
```bash
# Required for AI analysis
export LITELLM_TOKEN="your_litellm_api_token"
```

**Config File:**
```json
{
  "litellm": {
    "enabled": true,
    "base_url": "https://api.openai.com",
    "model": "gpt-4o",
    "timeout_seconds": 60,
    "analysis_word_limit": 100
  }
}
```

**Note**: AI analysis is completely optional. If not configured, the tool generates comprehensive reports without AI insights.

### Usage

When enabled, AI analysis is automatically included in generated reports:

```bash
# AI analysis will be included if configured
./github-okr-fetcher

# Output will show:
# 🤖 LiteLLM analysis enabled with model: gpt-4
# 🔍 Analyzing OKR data with AI...
# ✅ AI analysis completed successfully
```

### Sample AI Output

```markdown
## 🤖 AI Analysis

### Success & Achievements
- Completed 3 of 5 infrastructure objectives
- Deployed ImageFlux migration framework
- Achieved 85% team adoption rate

### Business Impact
- **Quantitative**: 40% reduction in deployment time, 15% improvement in system reliability
- **Qualitative**: Enhanced developer productivity, improved system observability
```

## 🏗️ Architecture

The project follows **Hexagonal Architecture** (Ports and Adapters) for clean separation of concerns:

```
github-okr-fetcher/
├── main.go              # Application entry point
├── cmd/                 # Cobra CLI commands
│   ├── root.go         # Root command and main logic
│   └── generate.go     # Generate config subcommand
├── internal/            # Core application code
│   ├── domain/          # Business logic and entities
│   │   ├── entity/      # Domain models
│   │   └── service/     # Domain services
│   ├── ports/           # Interface definitions
│   └── adapters/        # External integrations
│       ├── github/      # GitHub API adapter
│       ├── config/      # Configuration adapter
│       ├── output/      # Output format adapters
│       └── litellm/     # AI analysis adapter
├── docs/                # Documentation
│   └── architecture.md # Detailed architecture diagram
└── pkg/                 # Shared utilities
```

### Key Components

- **Domain Layer**: Core business logic for OKR processing
- **Ports**: Interfaces defining contracts between layers
- **Adapters**: Implementations for external services (GitHub API, file I/O, AI analysis)
- **Services**: Orchestration of domain logic

For a detailed architecture diagram and explanation, see [docs/architecture.md](docs/architecture.md).

## 📋 OKR Structure & Smart Status Detection

### Issue Classification

The tool automatically classifies GitHub issues into:

1. **Objectives**: Top-level goals with no parent reference
2. **Key Results**: Measurable outcomes linked to objectives via explicit references

### Intelligent Status Detection

#### **KR Status Prioritization**
The tool uses sophisticated logic to determine Key Result status:

1. **Latest Weekly Update Symbols**: Searches through all weekly updates to find the most recent one with valid status symbols
2. **Symbol Detection**: Recognizes status emojis (🟢🟡🔴⚠️🚫✅) and keywords ("completed", "blocked", "on track", etc.)
3. **Business Rules**: Applies smart logic (e.g., closed GitHub issues = completed, "completed" + open = on-track)
4. **Fallback Chain**: Weekly update symbols → LatestUpdate.Status → Unknown (only if no updates exist)

#### **Objective Status Aggregation**
Objectives automatically derive their status from child Key Results:

- **Priority Order**: Blocked > Delayed > At-Risk > Caution > Completed > On-Track > Unknown
- **Completion Logic**: All KRs completed = Objective completed
- **Progress Logic**: ≥50% KRs completed = Objective on-track
- **Risk Propagation**: Any high-priority status (blocked/delayed) overrides objective status

### Hierarchy Detection

The tool uses explicit reference detection for reliable parent-child relationships:

1. **Explicit References**: Looks for "Parent Issue: #123" or "Parent Issue: https://github.com/.../issues/123" in issue title/body
2. **Clean Detection**: Simplified approach ensures reliable relationship mapping
3. **Automatic Classification**: Issues with parent references become Key Results, others become Objectives

### Example Structure with Smart Status

```
📌 🟢 Objective: Drive Infrastructure Modernization (#25497) [on-track]
├── 📊 ✅ KR1: Enable ImageFlux migration (#25498) [completed]
├── 📊 🟢 KR2: Deploy gateways via WSCD (#25499) [on-track] 
└── 📊 🟡 KR3: Standardize Istio usage (#25519) [caution]
```

### Weekly Update Format

The tool recognizes and parses weekly updates in comments:

```markdown
# Weekly update 2025-07-09

## 📊 Status
- Progress: On-track 🟢
- Scope: Clear

## 🎯 Goals
- Complete feature implementation
- Deploy to staging environment

## 💡 Key Points
- Successfully completed API integration
- Performance tests show 40% improvement

## ✅ Completed
- API endpoint implementation
- Unit test coverage

## 🏃 In Progress
- Integration testing
- Documentation updates

## 🗒 Notes
- Stakeholder feedback very positive
```

#### **Status Detection from Updates**
- **Emoji Recognition**: 🟢 (on-track), 🟡 (caution), 🔴 (delayed), ⚠️ (at-risk), 🚫 (blocked), ✅ (completed)
- **Keyword Detection**: "completed", "done", "blocked", "delayed", "on track", "at risk"
- **Context Analysis**: Considers both explicit status declarations and content analysis
- **Structured Parsing**: Extracts goals, progress, completed items, and notes from formatted sections

## 🛠️ Development

### Running Tests

```bash
go test ./...
```

### Building for Different Platforms

```bash
# macOS
GOOS=darwin GOARCH=amd64 go build -o github-okr-fetcher-mac

# Linux
GOOS=linux GOARCH=amd64 go build -o github-okr-fetcher-linux

# Windows
GOOS=windows GOARCH=amd64 go build -o github-okr-fetcher.exe
```

### Code Structure

- Follow Go best practices and conventions
- Use interfaces for dependency injection
- Keep domain logic separate from infrastructure
- Write unit tests for business logic

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🙏 Acknowledgments

- Built with [go-github](https://github.com/google/go-github) library
- Inspired by OKR tracking best practices
- Thanks to all contributors

## 📞 Support

For issues, questions, or contributions, please open an issue on GitHub. 