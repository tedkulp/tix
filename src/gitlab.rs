use gitlab::api::{projects, users, Query};
use gitlab::Gitlab;
use serde::Deserialize;

#[derive(Debug)]
pub struct GitlabProject {
    client: Gitlab,
    name: String,
}

#[derive(Debug, Deserialize)]
#[allow(dead_code)]
pub struct GitlabUser {
    pub id: u64,
}

#[derive(Debug, Deserialize)]
#[allow(dead_code)]
pub struct GitlabMilestone {
    pub id: u64,
    pub name: String,
}

#[derive(Debug, Deserialize)]
#[allow(dead_code)]
pub struct GitlabGroup {
    pub id: u64,
    pub name: String,
    pub milestones: Vec<GitlabMilestone>,
}

#[derive(Debug, Deserialize)]
#[allow(dead_code)]
pub struct GitlabIssue {
    pub id: u32,
    pub iid: u32,
    pub title: String,
}

impl GitlabProject {
    pub fn new(project_name: String) -> Self {
        let token = std::env::var("GITLAB_TOKEN").expect("GITLAB_TOKEN env variable is required");
        let client = Gitlab::new("gitlab.com", token).unwrap();

        Self {
            client,
            name: project_name,
        }
    }

    pub fn current_user(&self) -> GitlabUser {
        let endpoint = users::CurrentUser::builder().build().unwrap();
        let user: GitlabUser = endpoint.query(&self.client).unwrap();
        user
    }

    pub fn create_issue(&self, title: &str, labels: &str) -> GitlabIssue {
        let lbls = labels.split(',').map(|l| l.trim().to_string());
        let current_user = self.current_user();

        let endpoint = projects::issues::CreateIssue::builder()
            .project(self.name.to_string())
            .title(title.to_string())
            .labels(lbls)
            .assignee_id(current_user.id)
            .build()
            .unwrap();

        let issue: GitlabIssue = endpoint.query(&self.client).unwrap();
        issue
    }
}
