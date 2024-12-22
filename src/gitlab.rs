use gitlab::api::{projects, Query};
use gitlab::Gitlab;
use serde::Deserialize;

#[derive(Debug)]
pub struct GitlabProject {
    client: Gitlab,
    name: String,
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
        let client = Gitlab::new("gitlab.com", std::env::var("GITLAB_TOKEN").unwrap()).unwrap();

        Self {
            client,
            name: project_name,
        }
    }

    pub fn create_issue(&self, title: String, labels: String) -> GitlabIssue {
        let lbls = labels.split(',').map(|l| l.trim().to_string());

        let endpoint = projects::issues::CreateIssue::builder()
            .project(self.name.to_string())
            .title(title.to_string())
            .labels(lbls)
            .build()
            .unwrap();

        let issue: GitlabIssue = endpoint.query(&self.client).unwrap();
        issue
    }

    // pub fn get_project(
    //     &self,
    //     project_name: &str,
    // ) -> Result<_, gitlab::api::ApiError<gitlab::RestError>> {
    //     let endpoint = projects::Project::builder()
    //         .project(project_name)
    //         .build()
    //         .unwrap();
    //
    //     // Call the endpoint. The return type decides how to represent the value.
    //     endpoint.query(&self.client)
    // }
}
