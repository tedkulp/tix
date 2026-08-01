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

// configFileOverride, when set via SetConfigFile, takes precedence over the
// default $HOME/.tix.yml lookup in Load.
var configFileOverride string

// SetConfigFile sets an explicit config file path for Load to use instead of
// the default $HOME/.tix.yml. Passing an empty string restores the default.
func SetConfigFile(path string) {
	configFileOverride = path
}

// Load reads the configuration from the specified file
func Load() (*Settings, error) {
	v := viper.New()
	if configFileOverride != "" {
		v.SetConfigFile(expandHomeDir(configFileOverride))
	} else {
		v.SetConfigName(".tix")
		v.SetConfigType("yaml")
		v.AddConfigPath("$HOME")
	}

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

// FindRepoForDir returns the code repository whose directory contains dir, or
// nil if none does. Non-code repos (those without a directory) are ignored.
// When repositories are nested, the most deeply nested match wins.
func (s *Settings) FindRepoForDir(dir string) *Repository {
	var match *Repository
	bestMatchLength := 0

	for i := range s.Repositories {
		repo := &s.Repositories[i]
		if !repo.IsCodeRepo() {
			continue
		}
		absRepoDir, err := filepath.Abs(repo.Directory)
		if err != nil {
			continue
		}
		if !dirContains(absRepoDir, dir) {
			continue
		}
		// Prefer the longest matching directory so nested repos win over parents.
		if len(absRepoDir) > bestMatchLength {
			match = repo
			bestMatchLength = len(absRepoDir)
		}
	}

	return match
}

// dirContains reports whether dir is parent itself or a directory beneath it.
// It compares whole path segments rather than raw strings, so a sibling that
// merely shares a name prefix (/src/tix-other vs /src/tix) is not a match.
func dirContains(parent, dir string) bool {
	parent = filepath.Clean(parent)
	dir = filepath.Clean(dir)
	if dir == parent {
		return true
	}
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent += string(filepath.Separator)
	}
	return strings.HasPrefix(dir, parent)
}
