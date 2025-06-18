package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// Worktree represents worktree configuration
type Worktree struct {
	DefaultBranch string `yaml:"default_branch" mapstructure:"default_branch"`
	Enabled       bool   `yaml:"enabled" mapstructure:"enabled"`
}

// Repository represents a single repository configuration
type Repository struct {
	Name          string   `yaml:"name" mapstructure:"name"`
	Directory     string   `yaml:"directory" mapstructure:"directory"`
	DefaultLabels string   `yaml:"default_labels" mapstructure:"default_labels"`
	ReadyLabel    string   `yaml:"ready_label" mapstructure:"ready_label"`
	GithubRepo    string   `yaml:"github_repo" mapstructure:"github_repo"`
	GitlabRepo    string   `yaml:"gitlab_repo" mapstructure:"gitlab_repo"`
	DefaultBranch string   `yaml:"default_branch" mapstructure:"default_branch"`
	Worktree      Worktree `yaml:"worktree,omitempty" mapstructure:"worktree"`
}

// Settings represents the root configuration
type Settings struct {
	ReadyLabel   string       `yaml:"ready_label" mapstructure:"ready_label"`
	Repositories []Repository `yaml:"repositories" mapstructure:"repositories"`
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

	// Expand home directory in paths
	for i := range settings.Repositories {
		repo := &settings.Repositories[i]
		repo.Directory = expandHomeDir(repo.Directory)
	}

	return &settings, nil
}

// expandHomeDir expands the home directory in a path
func expandHomeDir(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Clean(filepath.Join(home, path[2:]))
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
