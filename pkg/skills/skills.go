package skills

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// InstallScriptURL is the official installer, the same one documented in https://github.com/Qovery/qovery-skills
	InstallScriptURL = "https://raw.githubusercontent.com/Qovery/qovery-skills/main/install.sh"
	// ManualInstallCommand is shown to users who cannot run the installer from the CLI (e.g. Windows without bash)
	ManualInstallCommand = "curl -fsSL https://skill.qovery.com/install.sh | bash"

	// routerSkill is installed with every other Qovery skill: finding it is enough to know the skills are there
	routerSkill = "qovery"
	versionFile = "_version.txt"
)

// Installation is one directory where the Qovery skills are installed
type Installation struct {
	Path    string
	Version string
}

// installBaseDirs mirrors the global and project locations written by install.sh
func installBaseDirs(home string, cwd string) []string {
	return []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".config", "opencode", "skills"),
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(cwd, ".claude", "skills"),
		filepath.Join(cwd, ".opencode", "skills"),
		filepath.Join(cwd, ".agents", "skills"),
	}
}

func findInstallations(home string, cwd string) []Installation {
	var installations []Installation
	seen := map[string]bool{}
	for _, base := range installBaseDirs(home, cwd) {
		dir := filepath.Join(base, routerSkill)
		// Running from the home directory makes the project locations the same as the global ones
		if seen[dir] {
			continue
		}
		seen[dir] = true

		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
			continue
		}
		version := "unknown"
		if content, err := os.ReadFile(filepath.Join(dir, versionFile)); err == nil && strings.TrimSpace(string(content)) != "" {
			version = strings.TrimSpace(string(content))
		}
		installations = append(installations, Installation{Path: dir, Version: version})
	}
	return installations
}

// FindInstallations returns every location where the Qovery skills are installed, globally or in the current project
func FindInstallations() []Installation {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = home
	}
	return findInstallations(home, cwd)
}

// agentTools are the AI coding tools that load skills from the locations install.sh writes to
var agentTools = []struct {
	name string
	dir  string
}{
	{"Claude Code", ".claude"},
	{"Cursor", ".cursor"},
	{"Codex", ".codex"},
	{"OpenCode", filepath.Join(".config", "opencode")},
	{"Gemini CLI", ".gemini"},
	{"Windsurf", filepath.Join(".codeium", "windsurf")},
}

// detectAgentTools lists the AI coding tools that have a configuration directory in the user's home
func detectAgentTools(home string) []string {
	var found []string
	for _, tool := range agentTools {
		if info, err := os.Stat(filepath.Join(home, tool.dir)); err == nil && info.IsDir() {
			found = append(found, tool.name)
		}
	}
	return found
}

// Install runs the official skills installer. With project set, the skills are installed in the current directory only
func Install(project bool, stdout io.Writer, stderr io.Writer) error {
	return runInstaller(installerArgs(project, false), stdout, stderr)
}

// Uninstall removes the Qovery skills from every known location
func Uninstall(stdout io.Writer, stderr io.Writer) error {
	return runInstaller(installerArgs(false, true), stdout, stderr)
}

func installerArgs(project bool, uninstall bool) []string {
	switch {
	case uninstall:
		return []string{"--uninstall"}
	case project:
		return []string{"--project"}
	default:
		return []string{"--global"}
	}
}

func runInstaller(args []string, stdout io.Writer, stderr io.Writer) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("the skills installer needs bash: run it from WSL or Git Bash with '%s'", ManualInstallCommand)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		return fmt.Errorf("the skills installer needs bash, which was not found in PATH: %w", err)
	}

	script, err := downloadInstaller()
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(script)
	}()

	cmd := exec.Command(bash, append([]string{script}, args...)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("the skills installer failed: %w", err)
	}
	return nil
}

// downloadInstaller saves the installer to a temporary file, so it is never piped into a shell
func downloadInstaller() (string, error) {
	client := http.Client{Timeout: 30 * time.Second}
	res, err := client.Get(InstallScriptURL)
	if err != nil {
		return "", fmt.Errorf("cannot download the skills installer: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cannot download the skills installer: %s returned %s", InstallScriptURL, res.Status)
	}

	// The installer is a few kilobytes: cap the read so a broken response cannot fill the disk
	content, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("cannot download the skills installer: %w", err)
	}
	if !strings.HasPrefix(string(content), "#!") {
		return "", errors.New("cannot download the skills installer: the response is not a shell script")
	}

	file, err := os.CreateTemp("", "qovery-skills-install-*.sh")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()
	if _, err := file.Write(content); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}
