use config::{Config, ConfigError};
use serde::Deserialize;

#[derive(Debug, Deserialize)]
#[allow(unused)]
pub struct Worktree {
    pub default_branch: String,
    pub enabled: bool,
}

fn default_worktree() -> Worktree {
    Worktree {
        default_branch: "main".to_string(),
        enabled: false,
    }
}

#[derive(Debug, Deserialize)]
#[allow(unused)]
pub struct Repository {
    #[serde(default)]
    pub default_labels: Option<String>,
    pub directory: String,
    pub name: String,
    pub github_repo: Option<String>,
    pub gitlab_repo: Option<String>,
    #[serde(default = "default_branch")]
    pub default_branch: String,
    #[serde(default = "default_worktree")]
    pub worktree: Worktree,
}

fn default_branch() -> String {
    "main".to_string()
}

#[derive(Debug, Deserialize)]
#[allow(unused)]
pub struct Settings {
    pub repositories: Vec<Repository>,
}

impl Settings {
    pub fn new() -> Result<Self, ConfigError> {
        let s = Config::builder()
            .add_source(
                config::File::with_name(&shellexpand::tilde("~/.tix"))
                    .format(config::FileFormat::Yaml),
            )
            .build()?;

        s.try_deserialize::<Settings>()
    }

    pub fn repo_names(&self) -> anyhow::Result<Vec<String>> {
        let blah = self
            .repositories
            .iter()
            .map(|repo| repo.name.clone())
            .collect::<Vec<String>>();
        println!("{:?}", blah);
        Ok(blah)
    }

    pub fn get_repo(&self, name: String) -> Option<&Repository> {
        self.repositories.iter().find(|x| name == x.name)
    }
}

impl Repository {
    pub fn get_git_basedir(&self) -> String {
        if self.worktree.enabled {
            shellexpand::tilde(format!("{}/{}", self.directory, self.default_branch).as_str())
                .into_owned()
        } else {
            shellexpand::tilde(self.directory.as_str()).into_owned()
        }
    }

    pub fn get_worktree_dir(&self, name: String) -> String {
        shellexpand::tilde(format!("{}/{}", self.directory, name).as_str()).into_owned()
    }
}
