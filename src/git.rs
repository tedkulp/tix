use anyhow::Error;

pub fn open_repo(repo: &crate::settings::Repository) -> Result<git2::Repository, git2::Error> {
    let path = repo.get_git_basedir();
    git2::Repository::open(path)
}

pub fn is_repo_clean(repo: &git2::Repository) -> Result<bool, Error> {
    match repo.state() {
        git2::RepositoryState::Clean => Ok(true),
        _ => Err(anyhow::anyhow!("repo is not clean")),
    }
}
