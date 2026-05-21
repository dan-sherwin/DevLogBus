package app

type Commands struct {
	BuildInfoCommandDef
	Emit        EmitCommand           `cmd:"" help:"Publish one structured log record"`
	Tail        TailCommand           `cmd:"" help:"Tail broker records"`
	Socket      SocketCommand         `cmd:"" help:"Print the configured default socket path"`
	Completions CompletionsCommandDef `cmd:"" name:"autoCompletions" help:"Manage shell completions"`
}
