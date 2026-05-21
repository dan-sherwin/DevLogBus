package app

type Commands struct {
	BuildInfoCommandDef
	Emit        EmitCommand           `cmd:"" help:"Publish one structured log record"`
	Tail        TailCommand           `cmd:"" help:"Tail broker records"`
	TUI         TUICommand            `cmd:"" name:"tui" help:"Open the interactive terminal log viewer"`
	Expunge     ExpungeCommand        `cmd:"" help:"Delete broker replay records"`
	Endpoint    EndpointCommand       `cmd:"" help:"Print the configured broker endpoint"`
	Completions CompletionsCommandDef `cmd:"" name:"autoCompletions" help:"Manage shell completions"`
}
