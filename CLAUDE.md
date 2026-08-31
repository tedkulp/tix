# Claude AI Development Guide for Tix

This document provides guidelines for AI assistants (like Claude) when working on the Tix project.

## Project Overview

Tix is a command-line tool for creating tickets and branches in Git repositories, with support for both GitHub and GitLab. It streamlines developer workflows by:

- Creating tickets/issues and automatically generating branches
- Creating merge requests/pull requests with AI-powered descriptions
- Managing issue status and labels (ready/not ready for review)
- Supporting cross-repository workflows and issue linking
- Integrating with OpenAI for intelligent description generation

## Architecture

### Code Organization

```
tix/
├── cmd/              # Command definitions (create, mr, setdesc, ready, etc.)
├── internal/
│   ├── config/       # Configuration file parsing and management
│   ├── git/          # Git operations and worktree support
│   ├── logger/       # Logging utilities (WARN/INFO/DEBUG levels)
│   ├── services/     # Core services (GitHub, GitLab, OpenAI integration)
│   ├── utils/        # Utility functions
│   └── version/      # Version information
├── pkg/
│   └── models/       # Shared data models
└── main.go           # Application entry point
```

### Key Services

- **GitHub Service** (`internal/services/github.go`): GitHub API integration for issues, PRs, and repositories
- **GitLab Service** (`internal/services/gitlab.go`): GitLab API integration with GraphQL support for issue status
- **OpenAI Service** (`internal/services/openai.go`): AI-powered description generation with RAG support for large diffs
- **Embeddings Service** (`internal/services/embeddings.go`): Vector embeddings and similarity search for RAG

### Technology Stack

- **Language**: Go
- **APIs**: GitHub REST API, GitLab REST/GraphQL API, OpenAI API
- **Build Tools**: GoReleaser for multi-platform releases
- **Package Manager**: Homebrew tap for macOS distribution

## Development Workflow

### Making Changes

1. **Read before modifying**: Always read files before making changes to understand existing patterns
2. **Follow Go conventions**: Use idiomatic Go code, proper error handling, and clear naming
3. **Test locally**: Run `make build` and `make lint` before committing
4. **Keep it simple**: Avoid over-engineering; only add what's necessary

### Code Style

- Use `gofmt` for formatting (automatically applied by linters)
- Follow existing error handling patterns
- Use granular logging (logger.Debug, logger.Info, logger.Warn, logger.Error)
- Keep functions focused and single-purpose

### Testing and Linting

```bash
# Run linting
make lint

# Auto-fix linting issues
make lint-fix

# Run tests
make test

# Build the project
make build

# Run locally
make run
```

The project uses `golangci-lint` with configuration in `.golangci.yml` (version 2 format).

## CHANGELOG.md Requirements

**CRITICAL**: The CHANGELOG.md file must be kept up to date. This is non-negotiable.

### When to Update CHANGELOG.md

1. **After any code change**: Update CHANGELOG.md immediately after making any functional changes to the codebase
2. **Before creating a release**: Always verify CHANGELOG.md is complete and accurate before tagging a new version
3. **During PR/MR review**: Include CHANGELOG.md updates as part of the changeset

### CHANGELOG.md Format

The project follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format:

```markdown
## [Unreleased]

## [X.Y.Z] - YYYY-MM-DD

### Added
- New features

### Changed
- Changes to existing functionality

### Fixed
- Bug fixes

### Removed
- Removed features
```

### CHANGELOG.md Update Process

When making changes:

1. **Add entry under `[Unreleased]`**: Document changes in the appropriate section (Added/Changed/Fixed/Removed)
2. **Be specific but concise**: Describe what changed and why, focusing on user-visible impacts
3. **Before release**: Move `[Unreleased]` content to a new version section with the release date
4. **Update version links**: Add comparison links at the bottom of the file

**Example workflow:**

```bash
# 1. Make code changes
# 2. Update CHANGELOG.md under [Unreleased]
# 3. Commit both together
git add internal/services/openai.go CHANGELOG.md
git commit -m "Update OpenAI prompt for concise descriptions"

# Before release:
# 4. Move [Unreleased] content to versioned section
# 5. Tag the release
git tag -a v0.6.3 -m "Release v0.6.3"
```

### Missing CHANGELOG.md Entry

If you discover a past release is missing CHANGELOG.md documentation:
1. Research the git history (`git log`, `git show`, `git diff`)
2. Add the missing version section with accurate information
3. Note in your commit that you're backfilling changelog entries
4. Include the update in the next release

## Release Process

Tix follows [Semantic Versioning](https://semver.org/):
- **MAJOR** (X.0.0): Breaking changes
- **MINOR** (0.X.0): New features (backwards compatible)
- **PATCH** (0.0.X): Bug fixes

### Creating a Release

**IMPORTANT**: Always update CHANGELOG.md before creating a release!

```bash
# 1. Verify CHANGELOG.md is up to date
# 2. Move [Unreleased] content to new version section
# 3. Update version comparison links
# 4. Commit CHANGELOG.md changes
git add CHANGELOG.md
git commit -m "Release vX.Y.Z: <brief description>"

# 5. Create and push the tag
git tag -a vX.Y.Z -m "Release vX.Y.Z: <brief description>"
git push origin main
git push origin vX.Y.Z
```

GitHub Actions will automatically:
- Build binaries for macOS, Linux, Windows (amd64 and arm64)
- Create a GitHub Release
- Update the Homebrew tap

### Manual Release

If needed:
```bash
export GITHUB_TOKEN=your_token
goreleaser release --clean
```

For local testing:
```bash
goreleaser release --snapshot --clean
```

## AI-Powered Features

### OpenAI Integration

The `setdesc` command generates descriptions for merge requests and issues using OpenAI's API:

- **Direct approach**: For diffs under 50,000 estimated tokens (`EstimateTokenCount` is `len/4`, so
  roughly 200,000 characters), sends the full diff directly to the model in `descriptionModel`
- **RAG approach**: For large diffs, uses embeddings (text-embedding-3-small) and vector search to retrieve relevant context

### Prompt Engineering

When modifying AI prompts (`internal/services/openai.go`):

1. **Be explicit about desired output**: Specify format, length, and content requirements
2. **Provide clear instructions**: Tell the model what to include AND what to skip
3. **Test with real data**: Use actual diffs from the project to validate changes
4. **Consider verbosity**: Balance detail with conciseness (users prefer brief, focused descriptions)

Current prompts:
- `buildMRPrompt()`: Generates MR/PR descriptions (Summary, For Developers, For Quality sections)
- `buildIssuePrompt()`: Generates issue descriptions (Summary, Rationale, Acceptance Criteria)

## Platform Support

### GitHub
- Issues, Pull Requests, and repository management
- Timeline API for efficient PR lookup
- Token required: `GITHUB_TOKEN`

### GitLab
- Issues, Merge Requests, and repository management
- GraphQL API for issue status updates
- Supports custom issue status values
- Token required: `GITLAB_TOKEN`

### Cross-Repository Workflows

Tix supports issue-only repositories (repositories without code):
- Branch names include project prefix when linking across repositories
- Format: `project-123-feature-name`
- Enables merge requests to reference issues in other repositories

## Common Patterns

### Error Handling
```go
if err != nil {
    return fmt.Errorf("descriptive context: %w", err)
}
```

### Logging
```go
logger.Debug("Detailed diagnostic info", map[string]any{"key": value})
logger.Info("User-facing status message")
logger.Warn("Warning about non-fatal issue")
logger.Error("Error message", map[string]any{"error": err})
```

### Provider Interface Pattern
`SCMProvider` (`internal/services/scm.go`) is the single interface over both hosts:
- `GitLabProvider`: GitLab implementation
- `GitHubProvider`: GitHub implementation

## Configuration

Users configure Tix via `~/.tix.yml`:
- Repository definitions (name, github_repo/gitlab_repo, directory)
- Default labels and branches
- Ready/unready labels and status values
- Worktree settings

When adding new configuration options:
1. Update `internal/config/config.go` structures
2. Add validation if needed
3. Document in README.md
4. Provide sensible defaults
5. Update `.tix.example.yml`

## Dependencies

Key dependencies (see `go.mod`):
- `gitlab.com/gitlab-org/api/client-go`: GitLab API client
- `github.com/google/go-github/v62`: GitHub API client
- `github.com/sashabaranov/go-openai`: OpenAI API client
- `github.com/spf13/cobra`: CLI framework
- `gopkg.in/yaml.v3`: YAML configuration parsing

## Questions or Issues

When uncertain about:
- **Architecture decisions**: Ask the user for preferences
- **Breaking changes**: Discuss impact and alternatives
- **New features**: Clarify requirements before implementing
- **CHANGELOG.md content**: Describe what you changed and ask for confirmation

## Summary for AI Assistants

When working on Tix:
1. ✅ **Always** update CHANGELOG.md after making changes
2. ✅ **Always** verify CHANGELOG.md is current before releases
3. ✅ Read code before modifying it
4. ✅ Run `make lint` and `make build` before committing
5. ✅ Follow existing patterns and conventions
6. ✅ Keep descriptions and output concise (users value brevity)
7. ✅ Test with real-world scenarios when possible
8. ❌ Don't skip CHANGELOG.md updates (this is critical!)
9. ❌ Don't over-engineer solutions
10. ❌ Don't make breaking changes without discussion
