package completions

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/riywo/loginshell"
)

const (
	markerStart = "# >>> %s completions >>>"
	markerEnd   = "# <<< %s completions <<<"
)

type Result struct {
	Shell      string
	TargetPath string
	ReloadHint string
	Changed    bool
}

func Install(appName, shellOverride, binPathOverride string) (*Result, error) {
	shellName, err := detectShell(shellOverride)
	if err != nil {
		return nil, err
	}

	binPath, err := resolveBinaryPath(binPathOverride)
	if err != nil {
		return nil, err
	}

	targetPath, reloadHint, err := target(shellName, appName)
	if err != nil {
		return nil, err
	}

	fragment, err := fragment(shellName, appName, binPath)
	if err != nil {
		return nil, err
	}

	if shellName == "fish" {
		if err := writeFile(targetPath, fragment); err != nil {
			return nil, err
		}
	} else {
		if err := upsertManagedBlock(targetPath, appName, managedBlock(appName, fragment)); err != nil {
			return nil, err
		}
	}

	return &Result{Shell: shellName, TargetPath: targetPath, ReloadHint: reloadHint, Changed: true}, nil
}

func Uninstall(appName, shellOverride string) (*Result, error) {
	shellName, err := detectShell(shellOverride)
	if err != nil {
		return nil, err
	}

	targetPath, reloadHint, err := target(shellName, appName)
	if err != nil {
		return nil, err
	}

	changed := false
	if shellName == "fish" {
		err = os.Remove(targetPath)
		if err == nil {
			changed = true
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove completions file %s: %w", targetPath, err)
		}
	} else {
		changed, err = removeManagedBlockFromFile(targetPath, appName)
		if err != nil {
			return nil, err
		}
	}

	return &Result{Shell: shellName, TargetPath: targetPath, ReloadHint: reloadHint, Changed: changed}, nil
}

func detectShell(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if shell, err := loginshell.Shell(); err == nil && shell != "" {
		return filepath.Base(shell), nil
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell), nil
	}
	return "", fmt.Errorf("could not determine login shell; use --shell")
}

func resolveBinaryPath(override string) (string, error) {
	if override != "" {
		path, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolve --bin-path: %w", err)
		}
		return path, nil
	}

	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	if strings.Contains(path, string(filepath.Separator)+"go-build"+string(filepath.Separator)) {
		return "", fmt.Errorf("current executable path %q is temporary; rerun with --bin-path pointing at the real binary", path)
	}
	return path, nil
}

func target(shellName, appName string) (targetPath, reloadHint string, err error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("determine home directory: %w", err)
	}

	switch shellName {
	case "bash":
		rcFile := ".bashrc"
		if runtime.GOOS == "darwin" {
			rcFile = ".bash_profile"
		}
		targetPath = filepath.Join(homeDir, rcFile)
	case "zsh":
		targetPath = filepath.Join(homeDir, ".zshrc")
	case "fish":
		targetPath = filepath.Join(homeDir, ".config", "fish", "conf.d", appName+"_completions.fish")
	default:
		return "", "", fmt.Errorf("unsupported shell %q", shellName)
	}

	return targetPath, "source " + shellQuote(targetPath), nil
}

func fragment(shellName, appName, binPath string) (string, error) {
	quotedBin := shellQuote(binPath)
	switch shellName {
	case "bash":
		return fmt.Sprintf("complete -C %s %s\n", quotedBin, appName), nil
	case "zsh":
		return fmt.Sprintf("autoload -U +X bashcompinit && bashcompinit\ncomplete -C %s %s\n", quotedBin, appName), nil
	case "fish":
		return fmt.Sprintf("function __complete_%s\n    set -lx COMP_LINE (commandline -cp)\n    test -z (commandline -ct)\n    and set COMP_LINE \"$COMP_LINE \"\n    %s\nend\ncomplete -f -c %s -a \"(__complete_%s)\"\n", appName, quotedBin, appName, appName), nil
	default:
		return "", fmt.Errorf("unsupported shell %q", shellName)
	}
}

func managedBlock(appName, fragment string) string {
	return fmt.Sprintf(
		"%s\n%s\n%s\n",
		fmt.Sprintf(markerStart, appName),
		strings.TrimRight(fragment, "\n"),
		fmt.Sprintf(markerEnd, appName),
	)
}

func upsertManagedBlock(targetPath, appName, block string) error {
	existing, err := readOptionalTextFile(targetPath)
	if err != nil {
		return err
	}
	updated, removed := removeManagedBlock(existing, appName)
	if removed {
		existing = updated
	}
	if strings.TrimSpace(existing) == "" {
		return writeFile(targetPath, block)
	}
	return writeFile(targetPath, strings.TrimRight(existing, "\n")+"\n\n"+block)
}

func removeManagedBlockFromFile(targetPath, appName string) (bool, error) {
	existing, err := readOptionalTextFile(targetPath)
	if err != nil {
		return false, err
	}
	updated, removed := removeManagedBlock(existing, appName)
	if !removed {
		return false, nil
	}
	return true, writeFile(targetPath, updated)
}

func readOptionalTextFile(targetPath string) (string, error) {
	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read completions file %s: %w", targetPath, err)
	}
	return string(data), nil
}

func writeFile(targetPath, contents string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create completions directory for %s: %w", targetPath, err)
	}
	if err := os.WriteFile(targetPath, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("write completions file %s: %w", targetPath, err)
	}
	return nil
}

func removeManagedBlock(existing, appName string) (string, bool) {
	startMarker := fmt.Sprintf(markerStart, appName)
	endMarker := fmt.Sprintf(markerEnd, appName)

	startIdx := strings.Index(existing, startMarker)
	if startIdx < 0 {
		return existing, false
	}
	relEndIdx := strings.Index(existing[startIdx:], endMarker)
	if relEndIdx < 0 {
		return existing, false
	}

	endIdx := startIdx + relEndIdx + len(endMarker)
	for endIdx < len(existing) && (existing[endIdx] == '\n' || existing[endIdx] == '\r') {
		endIdx++
	}

	prefix := strings.TrimRight(existing[:startIdx], "\n")
	suffix := strings.TrimLeft(existing[endIdx:], "\n")

	switch {
	case prefix == "" && suffix == "":
		return "", true
	case prefix == "":
		return suffix, true
	case suffix == "":
		return prefix + "\n", true
	default:
		return prefix + "\n\n" + suffix, true
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
