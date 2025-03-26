# Tix

A CLI tool for creating tickets and branches in Git repositories, with support for both GitHub and GitLab.

## Features

- Create tickets in GitHub or GitLab
- Automatically create and checkout branches
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
```

## License

MIT 