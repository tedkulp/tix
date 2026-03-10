package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// WorktreeConfig represents worktree configuration
type WorktreeConfig struct {
	Path          string `yaml:"path" mapstructure:"path"`
	DefaultBranch string `yaml:"default_branch" mapstructure:"default_branch"`
}

// Repository represents a single repository configuration
type Repository struct {
	Name          string         `yaml:"name" mapstructure:"name"`
	Directory     string         `yaml:"directory" mapstructure:"directory"`
	DefaultLabels string         `yaml:"default_labels" mapstructure:"default_labels"`
	ReadyLabel    string         `yaml:"ready_label" mapstructure:"ready_label"`
	ReadyStatus   string         `yaml:"ready_status" mapstructure:"ready_status"`
	UnreadyLabel  string         `yaml:"unready_label" mapstructure:"unready_label"`
	UnreadyStatus string         `yaml:"unready_status" mapstructure:"unready_status"`
	GithubRepo    string         `yaml:"github_repo" mapstructure:"github_repo"`
	GitlabRepo    string         `yaml:"gitlab_repo" mapstructure:"gitlab_repo"`
	DefaultBranch string         `yaml:"default_branch" mapstructure:"default_branch"`
	Worktree      WorktreeConfig `yaml:"worktree,omitempty" mapstructure:"worktree"`
}

// Settings represents the root configuration
type Settings struct {
	ReadyLabel    string         `yaml:"ready_label" mapstructure:"ready_label"`
	ReadyStatus   string         `yaml:"ready_status" mapstructure:"ready_status"`
	UnreadyLabel  string         `yaml:"unready_label" mapstructure:"unready_label"`
	UnreadyStatus string         `yaml:"unready_status" mapstructure:"unready_status"`
	Worktree      WorktreeConfig `yaml:"worktree,omitempty" mapstructure:"worktree"`
	Repositories  []Repository   `yaml:"repositories" mapstructure:"repositories"`
}

// ResolveWorktreePath returns the worktree base path for a repo.
// Resolution order: per-repo > global > default (<repo-dir>/.worktrees)
func (s *Settings) ResolveWorktreePath(repo *Repository) string {
	if repo.Worktree.Path != "" {
		return expandHomeDir(repo.Worktree.Path)
	}
	if s.Worktree.Path != "" {
		return expandHomeDir(s.Worktree.Path)
	}
	return filepath.Join(repo.Directory, ".worktrees")
}

// ResolveDefaultBranch returns the default branch for a repo.
// Resolution order: per-repo worktree > per-repo default branch > global worktree > "main"
func (s *Settings) ResolveDefaultBranch(repo *Repository) string {
	if repo.Worktree.DefaultBranch != "" {
		return repo.Worktree.DefaultBranch
	}
	if repo.DefaultBranch != "" {
		return repo.DefaultBranch
	}
	if s.Worktree.DefaultBranch != "" {
		return s.Worktree.DefaultBranch
	}
	return "main"
}

// Load reads the configuration from the specified file
func Load() (*Settings, error) {
	v := viper.New()
	v.SetConfigName(".tix")
	v.SetConfigType("yaml")
	v.AddConfigPath("$HOME")

	// Enable env var substitution
	v.AutomaticEnv()

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var settings Settings
	decoderConfig := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &settings,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(v.AllSettings()); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand home directory in paths (only if directory is set)
	for i := range settings.Repositories {
		repo := &settings.Repositories[i]
		if repo.Directory != "" {
			repo.Directory = expandHomeDir(repo.Directory)
		}
	}

	return &settings, nil
}

// expandHomeDir expands the home directory in a path
func expandHomeDir(path string) string {
	if path == "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	// Only expand if path starts with ~/
	if strings.HasPrefix(path, "~/") {
		return filepath.Clean(filepath.Join(home, path[2:]))
	}
	return path
}

// IsCodeRepo returns true if the repository has a directory configured (i.e., it's a code repo)
func (r *Repository) IsCodeRepo() bool {
	return r.Directory != ""
}

// GetRepoNames returns a list of repository names
func (s *Settings) GetRepoNames() []string {
	names := make([]string, len(s.Repositories))
	for i, repo := range s.Repositories {
		names[i] = repo.Name
	}
	return names
}

// GetRepo returns a repository by name
func (s *Settings) GetRepo(name string) *Repository {
	for i := range s.Repositories {
		if s.Repositories[i].Name == name {
			return &s.Repositories[i]
		}
	}
	return nil
}
