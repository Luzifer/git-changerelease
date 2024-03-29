package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"

	"github.com/Luzifer/go_helpers/v2/env"
	"github.com/Luzifer/rconfig/v2"
)

const (
	fileModeChangelog = 0o644
	fileModeConfig    = 0o600
)

var (
	cfg = struct {
		ChangelogFile string `flag:"changelog" default:"History.md" description:"File to write the changelog to"`
		ConfigFile    string `flag:"config" default:"~/.git_changerelease.yaml" description:"Location of the configuration file"`
		LogLevel      string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		MkConfig      bool   `flag:"create-config" default:"false" description:"Copy an example configuration file to the location of --config"`
		NoEdit        bool   `flag:"no-edit" default:"false" description:"Do not open the $EDITOR to modify the changelog"`

		PreRelease  string `flag:"pre-release" default:"" description:"Pre-Release information to append to the version (i.e. 'beta' or 'alpha.1')"`
		ReleaseMeta string `flag:"release-meta" default:"" description:"Release metadata to append to the version (i.e. 'exp.sha.5114f85' or '20130313144700')"`

		VersionAndExit bool `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	config  *configFile
	version = "dev"

	matchers = make(map[*regexp.Regexp]semVerBump)

	errExitZero = errors.New("should exit zero now")
)

func initApp() (err error) {
	rconfig.AutoEnv(true)
	if err = rconfig.Parse(&cfg); err != nil {
		return fmt.Errorf("parsing cli options: %w", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("git-changerelease %s\n", version) //nolint:forbidigo // Fine in this case
		return errExitZero
	}

	cfg.ConfigFile, err = homedir.Expand(cfg.ConfigFile)
	if err != nil {
		return fmt.Errorf("expanding file path: %w", err)
	}

	var l logrus.Level
	if l, err = logrus.ParseLevel(cfg.LogLevel); err != nil {
		return fmt.Errorf("parsing log-level: %w", err)
	}
	logrus.SetLevel(l)

	if cfg.MkConfig {
		if err = os.WriteFile(cfg.ConfigFile, mustAsset("assets/git_changerelease.yaml"), fileModeConfig); err != nil {
			return fmt.Errorf("writing example config to %q: %w", cfg.ConfigFile, err)
		}
		logrus.Infof("wrote an example configuration to %q", cfg.ConfigFile)
		return errExitZero
	}

	if !cfg.NoEdit && os.Getenv("EDITOR") == "" {
		return errors.New("tried to open the changelog in the editor but there is no $EDITOR in your env")
	}

	projectConfig, err := filenameInGitRoot(".git_changerelease.yaml")
	if err != nil {
		return fmt.Errorf("building filename for project config: %w", err)
	}

	if config, err = loadConfig(cfg.ConfigFile, projectConfig); err != nil {
		return fmt.Errorf("loading config file(s): %w", err)
	}

	// Collect matchers
	if err = loadMatcherRegex(config.MatchPatch, semVerBumpPatch); err != nil {
		return fmt.Errorf("loading patch matcher: %w", err)
	}

	if err = loadMatcherRegex(config.MatchMajor, semVerBumpMajor); err != nil {
		return fmt.Errorf("loading major matcher: %w", err)
	}

	if cfg.ChangelogFile, err = filenameInGitRoot(cfg.ChangelogFile); err != nil {
		return fmt.Errorf("getting absolute path to changelog file: %w", err)
	}

	return nil
}

func loadMatcherRegex(matches []string, bump semVerBump) error {
	for _, match := range matches {
		r, err := regexp.Compile(match)
		if err != nil {
			return fmt.Errorf("parsing regex %q: %w", match, err)
		}
		matchers[r] = bump
	}

	return nil
}

func main() {
	var err error
	if err = initApp(); err != nil {
		if errors.Is(err, errExitZero) {
			os.Exit(0)
		}
		logrus.WithError(err).Fatal("initializing app")
	}

	// Get last tag
	lastTag, err := gitSilent("describe", "--tags", "--abbrev=0", `--match=v[0-9]*\.[0-9]*\.[0-9]*`)
	if err != nil {
		lastTag = "0.0.0"
	}

	logs, err := fetchGitLogs(lastTag, err != nil)
	if err != nil {
		logrus.WithError(err).Fatal("fetching git logs")
	}

	if len(logs) == 0 {
		logrus.Info("found no changes since last tag, stopping now.")
		return
	}

	// Generate new version
	newVersion, err := newVersionFromLogs(lastTag, logs)
	if err != nil {
		logrus.WithError(err).Fatal("bumping version")
	}

	// Render log
	if newVersion, err = renderLogAndGetVersion(newVersion, logs); err != nil {
		logrus.WithError(err).Fatal("writing changelog")
	}

	for _, pc := range config.PreCommitCommands {
		if err = runUserCommand(pc, map[string]string{"TAG_VERSION": newVersion.String()}); err != nil {
			logrus.WithError(err).WithField("cmd", pc).Fatal("executing pre-commit-commands")
		}
	}

	// Write the tag
	if err = applyTag("v" + newVersion.String()); err != nil {
		logrus.WithError(err).Fatal("applying tag")
	}
}

func newVersionFromLogs(lastTag string, logs []commit) (*semVer, error) {
	// Tetermine increase type
	semVerBumpType, err := selectBumpType(logs)
	if err != nil {
		return nil, fmt.Errorf("determining bump type: %w", err)
	}

	// Generate new version
	newVersion, err := parseSemVer(lastTag)
	if err != nil {
		return nil, fmt.Errorf("parsing previous version: %w", err)
	}
	newVersion.Bump(semVerBumpType)
	if err = newVersion.SetPrerelease(cfg.PreRelease); err != nil {
		return newVersion, fmt.Errorf("setting prerelease: %w", err)
	}
	if err = newVersion.SetMetadata(cfg.ReleaseMeta); err != nil {
		return newVersion, fmt.Errorf("setting metadata: %w", err)
	}

	return newVersion, nil
}

func readChangelog() (string, error) {
	if _, err := os.Stat(cfg.ChangelogFile); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logrus.Warn("changelog file does not yet exist, creating one")
			return "", nil
		}
		return "", fmt.Errorf("getting file stat: %w", err)
	}

	d, err := os.ReadFile(cfg.ChangelogFile)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	return string(d), nil
}

func renderLogAndGetVersion(newVersion *semVer, logs []commit) (*semVer, error) {
	oldLog, err := readChangelog()
	if err != nil {
		return nil, fmt.Errorf("reading old changelog: %w", err)
	}

	c, err := renderTemplate("log_template", mustAsset("assets/log_template.md"), struct {
		NextVersion *semVer
		Now         time.Time
		LogLines    []commit
		OldLog      string
	}{
		NextVersion: newVersion,
		Now:         time.Now(),
		LogLines:    logs,
		OldLog:      strings.TrimSpace(oldLog),
	})
	if err != nil {
		return nil, fmt.Errorf("rendering log: %w", err)
	}

	if err = os.WriteFile(cfg.ChangelogFile, c, fileModeChangelog); err != nil {
		return nil, fmt.Errorf("writing changelog: %w", err)
	}

	// Spawning editor
	if !cfg.NoEdit {
		editor := exec.Command(os.Getenv("EDITOR"), cfg.ChangelogFile) //#nosec:G204 // This is intended to use OS editor with configured changelog file
		editor.Stdin = os.Stdin
		editor.Stdout = os.Stdout
		editor.Stderr = os.Stderr
		if err = editor.Run(); err != nil {
			return nil, fmt.Errorf("editor process caused error: %w", err)
		}
	}

	// Read back version from changelog file
	newLog, err := readChangelog()
	if err != nil {
		return nil, fmt.Errorf("reading back changelog after edit: %w", err)
	}

	changelog := strings.Split(newLog, "\n")
	if len(changelog) < 1 {
		return nil, errors.New("changelog is empty, no way to read back the version")
	}

	newVersion, err = parseSemVer(strings.Split(changelog[0], " ")[1])
	if err != nil {
		return nil, fmt.Errorf("parsing new version from log: %w", err)
	}

	return newVersion, nil
}

func renderTemplate(name string, tplSrc []byte, values any) ([]byte, error) {
	tpl, err := template.New(name).Parse(string(tplSrc))
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, values); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

	return buf.Bytes(), nil
}

func runUserCommand(command string, extraEnv map[string]string) error {
	if len(command) == 0 {
		return errors.New("empty command specified")
	}

	// Check whether command is prefixed with a minus: If so the error
	// will only be logged and is not fatal to the changerelease run
	fatal := command[0] != '-'
	if !fatal {
		command = command[1:]
	}

	logrus.WithField("fatal", fatal).WithField("command", command).Trace("running pre_commit_commands")

	envVars := env.ListToMap(os.Environ())
	for k, v := range extraEnv {
		envVars[k] = v
	}

	wd, err := filenameInGitRoot(".")
	if err != nil {
		return fmt.Errorf("determining workdir for commands: %w", err)
	}

	cmd := exec.Command("/usr/bin/env", "bash", "-ec", command)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Env = env.MapToList(envVars)
	cmd.Dir = wd

	if err = cmd.Run(); err != nil {
		if fatal {
			return fmt.Errorf("command had error: %w", err)
		}
		logrus.WithError(err).WithField("cmd", command).Warn("command had error, marked as non-fatal")
	}

	return nil
}
