# Tix

A CLI tool for creating tickets and branches in Git repositories, with support for both GitHub and GitLab.

## Features

- Create tickets in GitHub or GitLab
- Automatically create and checkout branches
- Create draft merge requests/pull requests
- Support for Git worktrees
- Interactive repository selection
- Configurable default labels
- YAML configuration

## Installation

```bash
go install github.com/tedkulp/tix@latest
```

## Configuration

Create a configuration file at `~/.tix.yml` with the following structure:

```yaml
repositories:
  - name: my-project
    github_repo: username/repo
    directory: ~/src/my-project
    default_labels: bug,enhancement
  - name: another-project
    gitlab_repo: group/project
    directory: ~/src/another-project
    default_labels: feature
    worktree:
      enabled: true
      default_branch: main
```

## Environment Variables

- `GITHUB_TOKEN`: GitHub API token (required for GitHub repositories)
- `GITLAB_TOKEN`: GitLab API token (required for GitLab repositories)

## Usage

```bash
# Create a new ticket and branch
tix create

# Create a new ticket with a specific title
tix create --title "Add new feature"

# Create a merge request/pull request for the current branch
tix mr
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