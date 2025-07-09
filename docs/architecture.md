# GitHub OKR Fetcher - Architecture

## Overview

The GitHub OKR Fetcher follows **Hexagonal Architecture** (also known as Ports and Adapters pattern) to achieve clean separation of concerns, testability, and maintainability.

## Architecture Diagram

```mermaid
graph TB
    %% External Systems
    subgraph "External Systems"
        GH[GitHub API]
        GDOCS[Google Docs API]
        LITELLM[LiteLLM API]
        FS[File System]
        ENV[Environment Variables]
    end

    %% CLI Layer
    subgraph "CLI Layer"
        CLI[Cobra CLI]
        ROOT[Root Command]
        GEN[Generate Config Command]
    end

    %% Application Core
    subgraph "Application Core (Hexagon)"
        subgraph "Domain Layer"
            ENT[Entities]
            SVC[Services]
            LOGIC[Business Logic]
        end
        
        subgraph "Ports (Interfaces)"
            GHPORT[GitHub Port]
            OUTPORT[Output Port]
            CFGPORT[Config Port]
            AIPORT[AI Analysis Port]
        end
    end

    %% Adapters Layer
    subgraph "Adapters Layer"
        subgraph "Input Adapters"
            GHCLIENT[GitHub Client]
            CFGLOADER[Config Loader]
        end
        
        subgraph "Output Adapters"
            MDWRITER[Markdown Writer]
            JSONWRITER[JSON Writer]
            GDWRITER[Google Docs Writer]
            AIWRITER[AI Analysis Writer]
        end
        
        subgraph "Infrastructure"
            CACHE[API Cache]
            RATELIMIT[Rate Limiter]
            OAUTH[OAuth2 Handler]
            STATS[Statistics Tracker]
        end
    end

    %% Data Flow
    CLI --> ROOT
    ROOT --> SVC
    GEN --> CFGLOADER
    
    SVC --> GHPORT
    SVC --> OUTPORT
    SVC --> AIPORT
    
    GHPORT --> GHCLIENT
    OUTPORT --> MDWRITER
    OUTPORT --> JSONWRITER
    OUTPORT --> GDWRITER
    AIPORT --> AIWRITER
    CFGPORT --> CFGLOADER
    
    GHCLIENT --> GH
    GDWRITER --> GDOCS
    AIWRITER --> LITELLM
    CFGLOADER --> FS
    CFGLOADER --> ENV
    
    GHCLIENT -.-> CACHE
    GHCLIENT -.-> RATELIMIT
    GHCLIENT -.-> STATS
    GDWRITER -.-> OAUTH

    %% Styling
    classDef external fill:#e1f5fe,stroke:#01579b,stroke-width:2px
    classDef cli fill:#f3e5f5,stroke:#4a148c,stroke-width:2px
    classDef domain fill:#e8f5e8,stroke:#1b5e20,stroke-width:2px
    classDef ports fill:#fff3e0,stroke:#e65100,stroke-width:2px
    classDef adapters fill:#fce4ec,stroke:#880e4f,stroke-width:2px
    classDef infra fill:#f1f8e9,stroke:#33691e,stroke-width:2px

    class GH,GDOCS,LITELLM,FS,ENV external
    class CLI,ROOT,GEN cli
    class ENT,SVC,LOGIC domain
    class GHPORT,OUTPORT,CFGPORT,AIPORT ports
    class GHCLIENT,CFGLOADER,MDWRITER,JSONWRITER,GDWRITER,AIWRITER adapters
    class CACHE,RATELIMIT,OAUTH,STATS infra
```

## Architecture Layers

### 1. CLI Layer
- **Cobra CLI**: Professional command-line interface
- **Root Command**: Main application entry point and orchestration
- **Generate Config**: Configuration file generation subcommand

### 2. Application Core (Hexagon)

#### Domain Layer
- **Entities**: Core business objects (Config, Issue, ProjectInfo, WeeklyUpdate)
- **Services**: Business logic orchestration
- **Status Detection**: Intelligent OKR status determination logic

#### Ports (Interfaces)
- **GitHub Port**: Interface for GitHub API operations
- **Output Port**: Interface for report generation
- **Config Port**: Interface for configuration management
- **AI Analysis Port**: Interface for LiteLLM integration

### 3. Adapters Layer

#### Input Adapters
- **GitHub Client**: GitHub API integration with rate limiting and caching
- **Config Loader**: JSON configuration and environment variable handling

#### Output Adapters
- **Markdown Writer**: Rich markdown report generation
- **JSON Writer**: Structured data export
- **Google Docs Writer**: Native Google Docs API integration with OAuth2
- **AI Analysis Writer**: LiteLLM integration for business insights

#### Infrastructure
- **API Cache**: In-memory response caching with TTL
- **Rate Limiter**: GitHub API rate limiting with retry mechanisms
- **OAuth2 Handler**: Google authentication with browser automation
- **Statistics Tracker**: API usage and performance metrics

## Key Design Principles

### 1. Dependency Inversion
- Core business logic depends only on interfaces (ports)
- External dependencies are injected through adapters
- Easy to test and mock external systems

### 2. Single Responsibility
- Each adapter has a single, well-defined responsibility
- Clear separation between data fetching, processing, and output
- Modular components that can be independently tested

### 3. Open/Closed Principle
- Easy to add new output formats by implementing the Output Port
- New data sources can be added through new adapters
- Core business logic remains unchanged when adding features

### 4. Configuration-Driven
- Behavior controlled through JSON configuration
- Environment variables for sensitive data
- Runtime configuration without code changes

## Data Flow

1. **CLI** receives user commands and flags
2. **Root Command** loads configuration and initializes services
3. **Domain Services** orchestrate business logic using ports
4. **GitHub Adapter** fetches issues and comments with caching/rate limiting
5. **AI Adapter** (optional) analyzes data for business insights
6. **Output Adapters** generate reports in requested formats
7. **Infrastructure** provides cross-cutting concerns (caching, auth, metrics)

## Benefits of This Architecture

### Testability
- Domain logic can be unit tested in isolation
- Adapters can be mocked for integration testing
- Clear boundaries make testing straightforward

### Maintainability
- Changes to external APIs only affect adapters
- Business logic is protected from infrastructure changes
- Code is organized by responsibility

### Extensibility
- New output formats: implement Output Port
- New data sources: implement GitHub Port
- New AI providers: implement AI Analysis Port

### Security
- Sensitive data (tokens) handled only in adapters
- Environment variable injection at the edges
- OAuth2 flows isolated in dedicated adapters

## Technology Stack

- **Language**: Go 1.23+
- **CLI Framework**: Cobra
- **HTTP Client**: Go standard library + go-github
- **OAuth2**: golang.org/x/oauth2
- **Rate Limiting**: golang.org/x/time/rate
- **JSON Processing**: Go standard library
- **External APIs**: GitHub v4, Google Docs v1, LiteLLM

This architecture ensures the GitHub OKR Fetcher is maintainable, testable, and extensible while following Go best practices and clean architecture principles.