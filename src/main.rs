mod args;
mod commands;
mod git;
mod gitlab;
mod settings;
mod util;

use args::{CommandType::Create, TixArgs};
use clap::Parser;
use commands::create::create;

fn main() -> anyhow::Result<()> {
    let cli = TixArgs::parse();

    match &cli.command {
        Create(create_command) => create(create_command)?,
    }

    Ok(())
}
