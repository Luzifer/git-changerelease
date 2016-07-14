package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
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
}

func parseCommit(line string) (*commit, error) {
	t := strings.Split(line, "\t")
	if len(t) != 4 {
		return nil, errors.New("Unexpected line format")
	}
	return &commit{
		ShortHash:   t[0],
		Subject:     t[1],
		AuthorName:  t[2],
		AuthorEmail: t[3],
	}, nil
}

func git(stderrEnabled bool, args ...string) (string, error) {
	buf := bytes.NewBuffer([]byte{})

	cmd := exec.Command("git", args...)
	cmd.Stdout = buf
	if stderrEnabled {
		cmd.Stderr = os.Stderr
	}
	err := cmd.Run()

	return strings.TrimSpace(buf.String()), err
}

func gitErr(args ...string) (string, error) {
	return git(true, args...)
}

func gitSilent(args ...string) (string, error) {
	return git(false, args...)
}
