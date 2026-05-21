package app

import (
	"github.com/dan-sherwin/devlogbus/internal/buildinfo"
	"github.com/dan-sherwin/devlogbus/internal/journalbridge/app/consts"
)

type (
	BuildInfoCommandDef struct {
		Buildinfo BuildInfoCommand `cmd:"" hidden:"" help:"Show build information"`
	}
	BuildInfoCommand struct{}
)

func (b *BuildInfoCommand) Run() error {
	buildinfo.Print(consts.APPNAME, consts.Version, consts.Commit, consts.BuildDate)
	return nil
}
