package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

var (
	gitLogFormat      string
	gitLogFormatParts = []string{
		`%h`,  // ShortHash
		`%s`,  // Subject
		`%an`, // AuthorName
		`%ae`, // AuthorEmail
	}
)

func init() {
	gitLogFormat = strings.Join(gitLogFormatParts, `%x09`)
}

type commit struct {
	ShortHash   string
	Subject     string
	AuthorName  string
	AuthorEmail string
	BumpType    semVerBump
}

func applyTag(stringVersion string) error {
	var err error
	if _, err = gitErr("add", cfg.ChangelogFile); err != nil {
		return fmt.Errorf("adding changelog file: %w", err)
	}

	commitMessage, err := renderTemplate("commitMessage", []byte(config.ReleaseCommitMessage), struct {
		Version string
	}{
		Version: stringVersion,
	})
	if err != nil {
		return fmt.Errorf("building commit message: %w", err)
	}
	if _, err := gitErr("commit", "-m", string(commitMessage)); err != nil {
		return fmt.Errorf("committing changelog: %w", err)
	}

	tagType := "-s" // By default use signed tags
	if config.DiableTagSigning {
		tagType = "-a" // If requested switch to annotated tags
	}

	if _, err := gitErr("tag", tagType, "-m", stringVersion, stringVersion); err != nil {
		return fmt.Errorf("tagging release: %w", err)
	}

	return nil
}

//revive:disable-next-line:flag-parameter // Fine in this case
func fetchGitLogs(since string, fetchAll bool) ([]commit, error) {
	// Fetch logs since last tag / since repo start
	logArgs := []string{"log", `--format=` + gitLogFormat, "--abbrev-commit"}
	if !fetchAll {
		logArgs = append(logArgs, fmt.Sprintf("%s..HEAD", since))
	}

	rawLogs, err := gitErr(logArgs...)
	if err != nil {
		return nil, fmt.Errorf("reading git log entries: %w", err)
	}

	logs := []commit{}

	for _, l := range strings.Split(rawLogs, "\n") {
		if l == "" {
			continue
		}
		pl, err := parseCommit(l)
		if err != nil {
			return nil, errors.New("git used an unexpected log format")
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

func filenameInGitRoot(name string) (string, error) {
	root, err := git(io.Discard, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolving repo root: %w", err)
	}

	return path.Join(root, name), nil
}

func git(errOut io.Writer, args ...string) (string, error) {
	buf := bytes.NewBuffer([]byte{})

	cmd := exec.Command("git", args...)
	cmd.Stdout = buf
	cmd.Stderr = errOut

	err := cmd.Run()

	return strings.TrimSpace(buf.String()), err
}

func gitErr(args ...string) (string, error) {
	return git(os.Stderr, args...)
}

func gitSilent(args ...string) (string, error) {
	return git(io.Discard, args...)
}

func parseCommit(line string) (*commit, error) {
	t := strings.Split(line, "\t")
	if len(t) != len(gitLogFormatParts) {
		return nil, errors.New("unexpected line format")
	}

	c := &commit{
		ShortHash:   t[0],
		Subject:     t[1],
		AuthorName:  t[2],
		AuthorEmail: t[3],
	}

	for rex, bt := range matchers {
		if rex.MatchString(c.Subject) && bt > c.BumpType {
			c.BumpType = bt
		}
	}

	if c.BumpType == semVerBumpUndecided {
		c.BumpType = semVerBumpMinor
	}

	return c, nil
}
