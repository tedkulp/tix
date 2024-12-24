use crate::util::split_on_comma_and_whitespace;
use octocrab::Octocrab;

#[allow(dead_code)]
pub struct GithubProject {
    pub name: String,
    pub username: String,
    pub repo_name: String,
    client: Octocrab,
}

#[allow(dead_code)]
pub struct GithubIssue {
    pub id: u64,
    pub title: String,
    pub number: u64,
}

impl GithubProject {
    pub fn new(project_name: String) -> Self {
        let (username, repo_name) = project_name.split_once("/").expect(
            "Github repo name is not valid. It shouold bein the form of '<username>/<repo_name>'",
        );

        let token = std::env::var("GITHUB_TOKEN").expect("GITHUB_TOKEN env variable is required");
        let octocrab = Octocrab::builder()
            .personal_token(token)
            .build()
            .expect("Could not create github API connection");

        Self {
            name: project_name.clone(),
            username: username.to_string(),
            repo_name: repo_name.to_string(),
            client: octocrab,
        }
    }

    pub async fn create_issue(&self, title: &str, labels: &str) -> GithubIssue {
        let issue = self
            .client
            .issues(&self.username, &self.repo_name)
            .create(title)
            .labels(split_on_comma_and_whitespace(labels))
            .send()
            .await
            .unwrap();

        GithubIssue {
            id: issue.id.into_inner(),
            number: issue.number,
            title: issue.title,
        }
    }
}
