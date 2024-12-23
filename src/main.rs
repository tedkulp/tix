mod args;
mod commands;
mod git;
mod github;
mod gitlab;
mod settings;
mod util;

use args::{CommandType::Create, TixArgs};
use clap::Parser;
use commands::create::create;

#[async_std::main]
async fn main() -> anyhow::Result<()> {
    let cli = TixArgs::parse();

    match &cli.command {
        Create(create_command) => create(create_command).await?,
    }

    Ok(())
}
