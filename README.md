# Tix

A CLI tool for creating tickets and branches in Git repositories, with support for both GitHub and GitLab.

## Features

- Create tickets in GitHub or GitLab
- Automatically create and checkout branches
- Create draft merge requests/pull requests
- Generate AI-powered descriptions for merge requests and issues
- Mark issues as ready/not ready with configurable labels and status
- Support for Git worktrees
- Interactive repository selection
- Auto-detect repository based on current directory
- Configurable default labels and milestone generation
- GitLab issue status updates via GraphQL
- Granular logging levels (WARN/INFO/DEBUG)
- YAML configuration

## Installation

```bash
go install github.com/tedkulp/tix@latest
```

## Configuration

Create a configuration file at `~/.tix.yml` with the following structure:

```yaml
# Global defaults
ready_label: "ready for review"
ready_status: "in_progress"  # GitLab only

repositories:
  - name: my-project
    github_repo: username/repo
    directory: ~/src/my-project
    default_labels: bug,enhancement
    default_branch: main
    ready_label: "needs-review"  # Override global default
  - name: another-project
    gitlab_repo: group/project
    directory: ~/src/another-project
    default_labels: feature
    ready_label: "review-ready"
    ready_status: "ready"  # GitLab issue status
    worktree:
      enabled: true
      default_branch: main
```

### Configuration Options

#### Global Options
- `ready_label`: Default label to add when marking issues as ready (default: "ready")
- `ready_status`: Default status to set for GitLab issues when marking as ready (GitLab only)

#### Repository Options
- `name`: Unique name for the repository
- `github_repo`: GitHub repository in format "owner/repo" (GitHub only)
- `gitlab_repo`: GitLab repository in format "group/project" (GitLab only)
- `directory`: Local directory path for the repository
- `default_labels`: Comma-separated list of labels to add to new issues
- `default_branch`: Default branch name (default: "main")
- `ready_label`: Repository-specific ready label (overrides global)
- `ready_status`: Repository-specific ready status for GitLab (overrides global)
- `worktree.enabled`: Enable Git worktree support
- `worktree.default_branch`: Default branch for worktrees

#### GitLab Status Updates
When using GitLab repositories, the `ready_status` configuration allows you to automatically update issue status when marking issues as ready. This uses GitLab's GraphQL API to set the issue state. Standard status values include:
- `opened` (default)
- `closed`

Note: GitLab also supports custom issue status values if configured in your project settings.

For GitHub repositories, status updates are silently ignored since GitHub doesn't support issue status fields.

## Environment Variables

- `GITHUB_TOKEN`: GitHub API token (required for GitHub repositories)
- `GITLAB_TOKEN`: GitLab API token (required for GitLab repositories)
- `OPENAI_API_KEY`: OpenAI API key (required for AI-powered descriptions)

## Usage

### Create a new ticket and branch

```bash
# Create a new ticket interactively
tix create

# Create a new ticket with a specific title
tix create --title "Add new feature"

# Create a new ticket and assign it to yourself
tix create --self-assign
```

### Create a merge request/pull request

```bash
# Create a merge request/pull request for the current branch
tix mr

# Create a draft merge request/pull request
tix mr --draft

# Use a specific remote (default is 'origin')
tix mr --remote upstream
```

### Mark issues as ready/not ready

```bash
# Mark an issue as ready using configured label and status
tix ready

# Mark an issue as not ready (removes the ready label)
tix unready

# Override the default ready label
tix ready --label "needs-review"

# Override the default ready status (GitLab only)
tix ready --status "ready"

# Override both label and status
tix ready --label "review-ready" --status "in_progress"
```

### Generate descriptions with AI

```bash
# Generate and update descriptions for the current merge request and issue
tix setdesc

# Only update the issue description
tix setdesc --only-issue

# Only update the merge/pull request description
tix setdesc --only-mr
```

### Show version information

```bash
# Display version information
tix version
```

## Options

Global options that can be used with any command:

```bash
# Default logging level (WARN) - only shows warnings and errors
tix command

# Enable INFO level logging
tix -v command

# Enable DEBUG level logging
tix -vv command

# Use a specific config file
tix --config /path/to/config.yml command
```

## License

MIT 

## Releasing

This project uses [GoReleaser](https://goreleaser.com/) to handle releases. To release a new version:

1. Create and push a new tag with the version (e.g., `v0.1.0`):
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

2. The GitHub Actions workflow will automatically build and release the binaries to GitHub Releases.

### Homebrew

Releases are automatically published to the [tedkulp/homebrew-tap](https://github.com/tedkulp/homebrew-tap) repository. To install via Homebrew:

```bash
brew tap tedkulp/tap
brew install tix
```

### Manual Release

If you need to create a release manually:

1. Install GoReleaser: `brew install goreleaser/tap/goreleaser`
2. Set your GitHub token: `export GITHUB_TOKEN=your_token`
3. Run GoReleaser: `goreleaser release --clean`

For local testing, you can run: `goreleaser release --snapshot --clean` 