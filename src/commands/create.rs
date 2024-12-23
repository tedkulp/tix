use crate::{
    args::CreateCommand,
    git::{is_repo_clean, open_repo},
    settings,
    util::truncate_and_dash_case,
};
use anyhow::Result;
use git2::{build::CheckoutBuilder, WorktreeAddOptions};
use inquire::{max_length, required, Select, Text};

pub async fn create(cmd: &CreateCommand) -> Result<()> {
    let settings = settings::Settings::new()?;

    let repo_name = Select::new("Select a repository:", settings.repo_names()?).prompt()?;
    let repo_config = settings
        .get_repo(repo_name)
        .expect("Could not load this repo's config");
    let git_repo = open_repo(repo_config)?;

    if (repo_config.github_repo.is_none() && repo_config.gitlab_repo.is_none())
        || (repo_config.github_repo.is_some() && repo_config.gitlab_repo.is_some())
    {
        panic!("You must specify either a GitHub repo OR a GitLab repo");
    }

    is_repo_clean(&git_repo)?;

    let mut title = cmd.title.clone().unwrap_or_default();
    if title.is_empty() {
        title = Text::new("Title of issue:")
            .with_validator(required!())
            .with_validator(max_length!(255))
            .prompt()?;
    } else {
        println!("Using title: {}", title);
    }

    let labels = Text::new("Labels (comma separated):").prompt()?;

    let mut branch_name = String::new();

    if repo_config.gitlab_repo.is_some() {
        let project = crate::gitlab::GitlabProject::new(repo_config.gitlab_repo.clone().unwrap());
        let issue = project.create_issue(&title, &labels);
        branch_name = format!("{}-{}", issue.iid, truncate_and_dash_case(&issue.title, 50));
    }

    if repo_config.github_repo.is_some() {
        let project = crate::github::GithubProject::new(repo_config.github_repo.clone().unwrap());
        let issue = project.create_issue(&title, &labels).await;
        branch_name = format!("{}-{}", issue.id, truncate_and_dash_case(&issue.title, 50));
    }

    // Lookup the base branch reference
    let base_branch_ref =
        git_repo.find_branch(&repo_config.default_branch, git2::BranchType::Local)?;

    // Get the commit the base branch is pointing to
    let base_commit = base_branch_ref.get().peel_to_commit()?;

    // Create the new branch
    let branch = git_repo.branch(&branch_name, &base_commit, false)?;

    if repo_config.worktree.enabled {
        // 1. Create the worktree (and create a branch)
        let mut worktree_add_options = WorktreeAddOptions::new();
        let base_ref = worktree_add_options.reference(Option::Some(branch.get()));

        let get_worktree_dir = repo_config.get_worktree_dir(branch_name.clone());
        let worktree_path = get_worktree_dir.as_str();

        git_repo.worktree(
            branch_name.as_str(),
            std::path::Path::new(worktree_path),
            Some(base_ref),
        )?;

        println!(
            "Worktree created: branch {} in {}",
            branch.name().unwrap().unwrap(),
            worktree_path
        );
    } else {
        // Set current HEAD to new branch HEAD
        git_repo.set_head(branch.get().name().unwrap())?;

        // Checkout the new branch
        let mut checkout_builder = CheckoutBuilder::new();
        git_repo.checkout_head(Some(&mut checkout_builder))?;

        println!("Branch created: {}", branch.name().unwrap().unwrap());
    }

    Ok(())
}
