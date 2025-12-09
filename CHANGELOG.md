# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.1] - 2025-12-09

### Fixed
- `setdesc` command now uses GitLab's `related_merge_requests` API for efficient MR lookup instead of fetching all open MRs
- GitHub PR lookup now uses timeline API to find linked PRs instead of searching through all open PRs
- Significantly improved performance when working with repositories that have many open MRs/PRs

## [0.6.0] - 2025-12-02

### Added
- `start` command to create branches from existing issues (supports cross-repo)
- Cross-repository issue linking with `project-123-branch-name` format
- Support for issue-only repositories (optional `directory` in config)
- Cross-repository references in MR descriptions (GitLab: `group/project#123`, GitHub: `owner/repo#123`)
- Interactive and argument-based workflows for `start` and `create` commands

### Changed
- Branch name parsing now supports optional project prefix
- `create` command can now specify issue and code repositories as arguments
- `mr`, `setdesc`, `ready`, and `unready` commands handle cross-repo scenarios
- Improved acronym handling in branch name generation (e.g., "IRSA" → "irsa")

### Fixed
- Panic when `directory` field is empty in repository configuration
- Repository selector defaulting in `create` command

## [0.5.0] - 2025-11-11

### Changed
- **BREAKING**: Migrated from deprecated OpenAI Assistants API to RAG (Retrieval-Augmented Generation) with embeddings
  - Automatically handles large diffs that exceed context windows
  - Uses `text-embedding-3-small` model for cost-effective embeddings
  - Implements in-memory vector search with cosine similarity
  - Smart fallback: diffs <50K characters use direct approach without RAG
- Updated OpenAI library to latest version

### Added
- New `--use-rag` flag for `setdesc` command to force RAG on/off for testing
- Comprehensive test suite for embeddings and vector search functionality
- Shared prompt builders for consistent AI generation across approaches

### Removed
- Deprecated OpenAI Assistants API resources (threads, assistants, file uploads)
- Manual resource setup/cleanup in `setdesc` command

## [0.4.1] - 2024

### Changed
- Updated OpenAI library dependency

## [0.4.0] - 2024

### Added
- `ready` command to mark issues as ready for review with configurable labels and status
- `unready` command to mark issues as not ready with optional unready labels
- GitLab issue status updates via GraphQL API
- Support for custom GitLab issue status values
- Repository-specific ready/unready label and status configurations
- Global default configurations for ready/unready labels and status

### Changed
- Enhanced logging with granular levels (WARN/INFO/DEBUG)
- Improved error messages and user feedback
- Refactored issue and merge request handling for better code organization

## [0.3.0] - 2024

### Added
- `setdesc` command to generate AI-powered descriptions for merge requests and issues
- OpenAI integration for automated description generation
- Support for both merge request and issue description updates
- Selective description updates with `--only-issue` and `--only-mr` flags

### Changed
- Enhanced interactive prompts for better user experience
- Improved merge request and issue handling

## [0.2.1] - 2024

### Fixed
- Build date variable in Makefile
- GoReleaser pipeline configurations
- Minor bug fixes and improvements

## [0.2.0] - 2024

### Added
- Merge request/pull request creation functionality (`mr` command)
- Draft merge request/pull request support with `--draft` flag
- Remote repository selection with `--remote` flag
- Support for both GitHub and GitLab merge/pull requests

### Changed
- Enhanced issue creation with better validation
- Improved versioning system

## [0.1.0] - 2024

### Added
- Initial release
- `create` command for creating tickets and branches
- Support for both GitHub and GitLab repositories
- Automatic branch creation and checkout
- Interactive repository selection
- Auto-detect repository based on current directory
- Git worktree support
- Configurable default labels and branches
- Milestone support for GitLab issues
- YAML configuration file support
- Self-assignment option for issues
- GoReleaser configuration for automated releases
- Homebrew tap support

### Infrastructure
- GitHub Actions workflow for automated releases
- golangci-lint configuration
- Comprehensive test suite
- Makefile for common development tasks

[Unreleased]: https://github.com/tedkulp/tix/compare/v0.6.1...HEAD
[0.6.1]: https://github.com/tedkulp/tix/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/tedkulp/tix/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/tedkulp/tix/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/tedkulp/tix/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/tedkulp/tix/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/tedkulp/tix/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/tedkulp/tix/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/tedkulp/tix/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/tedkulp/tix/releases/tag/v0.1.0

