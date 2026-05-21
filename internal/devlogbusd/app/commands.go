package app

type Commands struct {
	BuildInfoCommandDef
	Completions CompletionsCommandDef `cmd:"" name:"autoCompletions" help:"Manage shell completions"`
}
