package buildinfo

import (
	"fmt"
	"runtime/debug"
	"strings"
)

func Print(appName, version, commit, buildDate string) {
	if bi, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("\nApp Name: %s\nGo Version: %s\nApp Version: %s\nCommit: %s\nBuild Date: %s\nPath: %s\nModule Version: %s\n", appName, bi.GoVersion, version, commit, buildDate, bi.Path, bi.Main.Version)
		for _, s := range bi.Settings {
			if strings.HasPrefix(s.Key, "-") {
				continue
			}
			fmt.Printf("%s: %s\n", s.Key, s.Value)
		}
		fmt.Printf("\n\n")
		return
	}
	fmt.Println("no build information available")
}
