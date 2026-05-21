package app

import (
	"fmt"

	"github.com/dan-sherwin/devlogbus/internal/completions"
	"github.com/dan-sherwin/devlogbus/internal/devlogbusd/app/consts"
)

type (
	CompletionsCommandDef struct {
		Install   InstallCompletionsCommand   `cmd:"" help:"Persistently install shell completions for the detected shell"`
		Uninstall UninstallCompletionsCommand `cmd:"" help:"Remove previously installed shell completions for the detected shell"`
	}
	InstallCompletionsCommand struct {
		Shell   string `name:"shell" help:"Override the detected login shell (bash|zsh|fish)"`
		BinPath string `name:"bin-path" help:"Absolute path to the binary to register for shell completions"`
	}
	UninstallCompletionsCommand struct {
		Shell string `name:"shell" help:"Override the detected login shell (bash|zsh|fish)"`
	}
)

func (c *InstallCompletionsCommand) Run() error {
	result, err := completions.Install(consts.APPNAME, c.Shell, c.BinPath)
	if err != nil {
		return err
	}
	fmt.Printf("Installed %s completions for %s in %s\n", result.Shell, consts.APPNAME, result.TargetPath)
	fmt.Printf("Restart your shell or run: %s\n", result.ReloadHint)
	return nil
}

func (c *UninstallCompletionsCommand) Run() error {
	result, err := completions.Uninstall(consts.APPNAME, c.Shell)
	if err != nil {
		return err
	}
	if !result.Changed {
		fmt.Printf("No installed %s completions were found for %s in %s\n", result.Shell, consts.APPNAME, result.TargetPath)
		return nil
	}
	fmt.Printf("Removed %s completions for %s from %s\n", result.Shell, consts.APPNAME, result.TargetPath)
	fmt.Printf("Restart your shell or run: %s\n", result.ReloadHint)
	return nil
}
