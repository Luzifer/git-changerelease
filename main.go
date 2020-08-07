package main

//go:generate make generate

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"

	"github.com/Luzifer/rconfig/v2"
)

var (
	cfg = struct {
		ChangelogFile string `flag:"changelog" default:"History.md" description:"File to write the changelog to"`
		ConfigFile    string `flag:"config" default:"~/.git_changerelease.yaml" description:"Location of the configuration file"`
		LogLevel      string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		MkConfig      bool   `flag:"create-config" default:"false" description:"Copy an example configuration file to the location of --config"`
		NoEdit        bool   `flag:"no-edit" default:"false" description:"Do not open the $EDITOR to modify the changelog"`

		PreRelease  string `flag:"pre-release" default:"" description:"Pre-Release information to append to the version (e.g. 'beta' or 'alpha.1')"`
		ReleaseMeta string `flag:"release-meta" default:"" description:"Release metadata to append to the version (e.g. 'exp.sha.5114f85' or '20130313144700')"`

		VersionAndExit bool `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	config  *configFile
	version = "dev"

	matchers = make(map[*regexp.Regexp]semVerBump)
)

func prepareRun() {
	var err error

	rconfig.AutoEnv(true)
	if err = rconfig.Parse(&cfg); err != nil {
		log.WithError(err).Fatal("Unable to parse commandline options")
	}

	if cfg.VersionAndExit {
		fmt.Printf("git-changerelease %s\n", version)
		os.Exit(0)
	}

	cfg.ConfigFile, err = homedir.Expand(cfg.ConfigFile)
	if err != nil {
		log.WithError(err).Fatal("Could not expand config file path")
	}

	var l log.Level
	if l, err = log.ParseLevel(cfg.LogLevel); err != nil {
		log.WithError(err).Fatal("Unable to parse log level")
	} else {
		log.SetLevel(l)
	}

	if cfg.MkConfig {
		if err = ioutil.WriteFile(cfg.ConfigFile, MustAsset("assets/git_changerelease.yaml"), 0600); err != nil {
			log.WithError(err).Fatalf("Could not write example configuration to %q", cfg.ConfigFile)
		}
		log.Infof("Wrote an example configuration to %q", cfg.ConfigFile)
		os.Exit(0)
	}

	if !cfg.NoEdit && os.Getenv("EDITOR") == "" {
		log.Fatal("You chose to open the changelog in the editor but there is no $EDITOR in your env")
	}

	if config, err = loadConfig(); err != nil {
		log.WithError(err).Fatal("Unable to load config file")
	}

	// Collect matchers
	if err = loadMatcherRegex(config.MatchPatch, semVerBumpPatch); err != nil {
		log.WithError(err).Fatal("Unable to load patch matcher expressions")
	}
	if err = loadMatcherRegex(config.MatchMajor, semVerBumpMajor); err != nil {
		log.WithError(err).Fatal("Unable to load major matcher expressions")
	}
}

func loadMatcherRegex(matches []string, bump semVerBump) error {
	for _, match := range matches {
		r, err := regexp.Compile(match)
		if err != nil {
			return fmt.Errorf("Unable to parse regex '%s': %s", match, err)
		}
		matchers[r] = bump
	}

	return nil
}

func readChangelog() string {
	if _, err := os.Stat(cfg.ChangelogFile); err != nil {
		log.Warn("Changelog file does not yet exist, creating one")
		return ""
	}

	d, err := ioutil.ReadFile(cfg.ChangelogFile)
	if err != nil {
		log.WithError(err).Fatal("Unable to read old changelog")
	}
	return string(d)
}

func quickTemplate(name string, tplSrc []byte, values map[string]interface{}) ([]byte, error) {
	tpl, err := template.New(name).Parse(string(tplSrc))
	if err != nil {
		return nil, errors.New("Unable to parse log template: " + err.Error())
	}
	buf := bytes.NewBuffer([]byte{})
	if err := tpl.Execute(buf, values); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func main() {
	prepareRun()

	// Get last tag
	lastTag, err := gitSilent("describe", "--tags", "--abbrev=0", `--match=v[0-9]*\.[0-9]*\.[0-9]*`)
	if err != nil {
		lastTag = "0.0.0"
	}

	logs, err := fetchGitLogs(lastTag, err != nil)
	if err != nil {
		log.WithError(err).Fatal("Could not fetch git logs")
	}

	if len(logs) == 0 {
		log.Info("Found no changes since last tag, stopping now.")
		return
	}

	// Generate new version
	newVersion, err := newVersionFromLogs(lastTag, logs)
	if err != nil {
		log.WithError(err).Fatal("Was unable to bump version")
	}

	// Render log
	if newVersion, err = renderLog(newVersion, logs); err != nil {
		log.WithError(err).Fatal("Could not write changelog")
	}

	// Write the tag
	if err = applyTag("v" + newVersion.String()); err != nil {
		log.WithError(err).Fatal("Unable to apply tag")
	}
}

func applyTag(stringVersion string) error {
	var err error
	if _, err = gitErr("add", cfg.ChangelogFile); err != nil {
		return fmt.Errorf("Unable to add changelog file: %s", err)
	}

	commitMessage, err := quickTemplate("commitMessage", []byte(config.ReleaseCommitMessage), map[string]interface{}{
		"Version": stringVersion,
	})
	if err != nil {
		return fmt.Errorf("Unable to compile commit message: %s", err)
	}
	if _, err := gitErr("commit", "-m", string(commitMessage)); err != nil {
		return fmt.Errorf("Unable to commit changelog: %s", err)
	}

	tagType := "-s" // By default use signed tags
	if config.DiableTagSigning {
		tagType = "-a" // If requested switch to annotated tags
	}

	if _, err := gitErr("tag", tagType, "-m", stringVersion, stringVersion); err != nil {
		return fmt.Errorf("Unable to tag release: %s", err)
	}

	return nil
}

func fetchGitLogs(since string, fetchAll bool) ([]commit, error) {
	// Fetch logs since last tag / since repo start
	logArgs := []string{"log", `--format=` + gitLogFormat, "--abbrev-commit"}
	if !fetchAll {
		logArgs = append(logArgs, fmt.Sprintf("%s..HEAD", since))
	}

	rawLogs, err := gitErr(logArgs...)

	if err != nil {
		return nil, fmt.Errorf("Unable to read git log entries: %s", err)
	}

	logs := []commit{}

	for _, l := range strings.Split(rawLogs, "\n") {
		if l == "" {
			continue
		}
		pl, err := parseCommit(l)
		if err != nil {
			return nil, errors.New("Git used an unexpected log format")
		}

		addLog := true
		for _, match := range config.IgnoreMessages {
			r := regexp.MustCompile(match)
			if r.MatchString(pl.Subject) {
				addLog = false
				break
			}
		}

		if addLog {
			logs = append(logs, *pl)
		}
	}

	return logs, nil
}

func renderLog(newVersion *semVer, logs []commit) (*semVer, error) {
	c, err := quickTemplate("log_template", MustAsset("assets/log_template.md"), map[string]interface{}{
		"NextVersion": newVersion,
		"Now":         time.Now(),
		"LogLines":    logs,
		"OldLog":      readChangelog(),
	})
	if err != nil {
		return nil, fmt.Errorf("Unable to compile log: %s", err)
	}

	if err = ioutil.WriteFile(cfg.ChangelogFile, bytes.TrimSpace(c), 0644); err != nil {
		return nil, fmt.Errorf("Unable to write new changelog: %s", err)
	}

	// Spawning editor
	if !cfg.NoEdit {
		editor := exec.Command(os.Getenv("EDITOR"), cfg.ChangelogFile)
		editor.Stdin = os.Stdin
		editor.Stdout = os.Stdout
		editor.Stderr = os.Stderr
		if err = editor.Run(); err != nil {
			return nil, errors.New("Editor ended with non-zero status, stopping here")
		}
	}

	// Read back version from changelog file
	changelog := strings.Split(readChangelog(), "\n")
	if len(changelog) < 1 {
		return nil, errors.New("Changelog is empty, no way to read back the version")
	}
	newVersion, err = parseSemVer(strings.Split(changelog[0], " ")[1])
	if err != nil {
		return nil, fmt.Errorf("Unable to parse new version from log: %s", err)
	}

	return newVersion, nil
}

func newVersionFromLogs(lastTag string, logs []commit) (*semVer, error) {
	// Tetermine increase type
	semVerBumpType, err := selectBumpType(logs)
	if err != nil {
		return nil, fmt.Errorf("Could not determine how to increase the version: %s", err)
	}

	// Generate new version
	newVersion, err := parseSemVer(lastTag)
	if err != nil {
		return nil, fmt.Errorf("Was unable to parse previous version: %s", err)
	}
	if newVersion.PreReleaseInformation == "" && cfg.PreRelease == "" {
		newVersion.Bump(semVerBumpType)
	}
	newVersion.PreReleaseInformation = cfg.PreRelease
	newVersion.MetaData = cfg.ReleaseMeta

	return newVersion, nil
}
