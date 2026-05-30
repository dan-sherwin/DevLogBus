package buildinfo

import (
	"fmt"
	"runtime/debug"
	"strings"
)

type Info struct {
	AppName       string `json:"appName"`
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	BuildDate     string `json:"buildDate"`
	GoVersion     string `json:"goVersion,omitempty"`
	ModulePath    string `json:"modulePath,omitempty"`
	ModuleVersion string `json:"moduleVersion,omitempty"`
}

func Read(appName, version, commit, buildDate string) Info {
	info := Info{
		AppName:   appName,
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = bi.GoVersion
		info.ModulePath = bi.Path
		info.ModuleVersion = bi.Main.Version
	}
	return info
}

func Print(appName, version, commit, buildDate string) {
	info := Read(appName, version, commit, buildDate)
	if bi, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("\nApp Name: %s\nGo Version: %s\nApp Version: %s\nCommit: %s\nBuild Date: %s\nPath: %s\nModule Version: %s\n", info.AppName, info.GoVersion, info.Version, info.Commit, info.BuildDate, info.ModulePath, info.ModuleVersion)
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

func PrintSummary(appName, version, commit, buildDate string) {
	info := Read(appName, version, commit, buildDate)
	fmt.Printf("%s %s\ncommit: %s\nbuild date: %s\n",
		info.AppName,
		valueOrUnknown(info.Version),
		valueOrUnknown(info.Commit),
		valueOrUnknown(info.BuildDate),
	)
}

func valueOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
