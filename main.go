package main

//go:generate go-bindata -o assets.go assets/

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/Luzifer/rconfig"
	"github.com/mitchellh/go-homedir"
)

type configFile struct {
	MatchPatch []string `yaml:"match_patch"`
	MatchMajor []string `yaml:"match_major"`
}

var (
	cfg = struct {
		ChangelogFile string `flag:"changelog" default:"History.md" description:"File to write the changelog to"`
		ConfigFile    string `flag:"config" default:"~/.git_changerelease.yaml" description:"Location of the configuration file"`
		MkConfig      bool   `flag:"create-config" default:"false" description:"Copy an example configuration file to the location of --config"`
		NoEdit        bool   `flag:"no-edit" default:"false" description:"Do not open the $EDITOR to modify the changelog"`

		PreRelease  string `flag:"pre-release" default:"" description:"Pre-Release information to append to the version (e.g. 'beta' or 'alpha.1')"`
		ReleaseMeta string `flag:"release-meta" default:"" description:"Release metadata to append to the version (e.g. 'exp.sha.5114f85' or '20130313144700')"`

		VersionAndExit bool `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	config  configFile
	version = "dev"
)

func init() {
	if err := rconfig.Parse(&cfg); err != nil {
		log.Fatalf("Unable to parse commandline options: %s", err)
	}

	if ecfg, err := homedir.Expand(cfg.ConfigFile); err != nil {
		log.Fatalf("Unable to parse config path: %s", err)
	} else {
		cfg.ConfigFile = ecfg
	}

	if cfg.VersionAndExit {
		fmt.Printf("git-changerelease %s\n", version)
		os.Exit(0)
	}

	if cfg.MkConfig {
		data, _ := Asset("assets/git_changerelease.yaml")
		ioutil.WriteFile(cfg.ConfigFile, data, 0600)
		log.Printf("Wrote an example configuration to %s", cfg.ConfigFile)
		os.Exit(0)
	}

	if !cfg.NoEdit && os.Getenv("EDITOR") == "" {
		log.Fatalf("You chose to open the changelog in the editor but there is no $EDITOR in your env")
	}

	if err := loadConfig(); err != nil {
		log.Fatalf("Unable to load config file: %s", err)
	}
}

func loadConfig() error {
	if _, err := os.Stat(cfg.ConfigFile); err != nil {
		return errors.New("Config file does not exist, use --create-config to create one")
	}

	data, err := ioutil.ReadFile(cfg.ConfigFile)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, &config)
}

func readChangelog() string {
	var changelog string

	if _, err := os.Stat(cfg.ChangelogFile); err == nil {
		d, err := ioutil.ReadFile(cfg.ChangelogFile)
		if err != nil {
			log.Fatalf("Unable to read old changelog: %s", err)
		}
		changelog = string(d)
	}

	return changelog
}

func main() {
	// Get last tag
	lastTag, err := gitSilent("describe", "--tags", "--abbrev=0")

	// Fetch logs since last tag / since repo start
	logArgs := []string{"log", `--format=` + gitLogFormat, "--abbrev-commit"}
	if err == nil {
		logArgs = append(logArgs, fmt.Sprintf("%s..HEAD", lastTag))
	} else {
		lastTag = "0.0.0"
	}
	rawLogs, err := gitErr(logArgs...)

	if err != nil {
		log.Fatalf("Unable to read git log entries: %s", err)
	}

	logs := []commit{}

	for _, l := range strings.Split(rawLogs, "\n") {
		if l == "" {
			continue
		}
		pl, err := parseCommit(l)
		if err != nil {
			log.Fatalf("Git used an unexpected log format.")
		}
		logs = append(logs, *pl)
	}

	if len(logs) == 0 {
		log.Printf("Found no changes since last tag, stopping now.")
		return
	}

	// Tetermine increase type
	semVerBumpType, err := selectBumpType(logs)
	if err != nil {
		log.Fatalf("Could not determine how to increase the version: %s", err)
	}

	// Generate new version
	newVersion, err := parseSemVer(lastTag)
	if err != nil {
		log.Fatalf("Was unable to parse previous version: %s", err)
	}
	if newVersion.PreReleaseInformation == "" && cfg.PreRelease == "" {
		newVersion.Bump(semVerBumpType)
	}
	newVersion.PreReleaseInformation = cfg.PreRelease
	newVersion.MetaData = cfg.ReleaseMeta

	// Render log
	rawTpl, _ := Asset("assets/log_template.md")
	tpl, err := template.New("log_template").Parse(string(rawTpl))
	if err != nil {
		log.Fatalf("Unable to parse log template: %s", err)
	}
	buf := bytes.NewBuffer([]byte{})
	tpl.Execute(buf, map[string]interface{}{
		"NextVersion": newVersion,
		"Now":         time.Now(),
		"LogLines":    logs,
		"OldLog":      readChangelog(),
	})

	if err := ioutil.WriteFile(cfg.ChangelogFile, bytes.TrimSpace(buf.Bytes()), 0644); err != nil {
		log.Fatalf("Unable to write new changelog: %s", err)
	}

	// Spawning editor
	if !cfg.NoEdit {
		editor := exec.Command(os.Getenv("EDITOR"), cfg.ChangelogFile)
		editor.Stdin = os.Stdin
		editor.Stdout = os.Stdout
		editor.Stderr = os.Stderr
		if err := editor.Run(); err != nil {
			log.Fatalf("Editor ended with non-zero status, stopping here.")
		}
	}

	// Read back version from changelog file
	changelog := strings.Split(readChangelog(), "\n")
	if len(changelog) < 1 {
		log.Fatalf("Changelog is empty, no way to read back the version.")
	}
	newVersion, err = parseSemVer(strings.Split(changelog[0], " ")[1])
	if err != nil {
		log.Fatalf("Unable to parse new version from log: %s", err)
	}

	// Write the tag
	if _, err := gitErr("add", cfg.ChangelogFile); err != nil {
		log.Fatalf("Unable to add changelog file: %s", err)
	}
	if _, err := gitErr("commit", "-m", "Prepared release v"+newVersion.String()); err != nil {
		log.Fatalf("Unable to commit changelog: %s", err)
	}
	if _, err := gitErr("tag", "-s", "-m", "v"+newVersion.String(), "v"+newVersion.String()); err != nil {
		log.Fatalf("Unable to tag release: %s", err)
	}
}
