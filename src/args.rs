use clap::{Args, Parser, Subcommand};

#[derive(Debug, Parser)]
#[clap(author, version, about)]
pub struct TixArgs {
    #[clap(subcommand)]
    pub command: CommandType,
}

#[derive(Debug, Subcommand)]
pub enum CommandType {
    /// Create a new ticket / branch
    Create(CreateCommand),
}

#[derive(Debug, Args)]
pub struct CreateCommand {
    /*
    This is positional
    pub name: String,
    */
    /// This is optional
    #[arg(short, long)]
    pub title: Option<String>,
}
