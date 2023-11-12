package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

type configFile struct {
	DiableTagSigning bool `yaml:"disable_signed_tags"`

	MatchMajor []string `yaml:"match_major"`
	MatchPatch []string `yaml:"match_patch"`

	ReleaseCommitMessage string `yaml:"release_commit_message"`

	IgnoreMessages []string `yaml:"ignore_messages"`

	PreCommitCommands []string `yaml:"pre_commit_commands"`
}

func loadConfig(configFiles ...string) (*configFile, error) {
	var err error

	c := &configFile{}
	if err = yaml.Unmarshal(mustAsset("assets/git_changerelease.yaml"), c); err != nil {
		return nil, fmt.Errorf("unmarshalling default config: %w", err)
	}

	for _, fn := range configFiles {
		if _, err = os.Stat(fn); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				logrus.WithField("path", fn).Debug("config-file does not exist, skipping")
				continue
			}
			return nil, fmt.Errorf("getting config-file stat for %q: %w", fn, err)
		}

		logrus.WithField("path", fn).Debug("loading config-file")

		dataFile, err := os.Open(fn) //#nosec:G304 // This is intended to load variable files
		if err != nil {
			return nil, fmt.Errorf("opening config file: %w", err)
		}

		if err = yaml.NewDecoder(dataFile).Decode(c); err != nil {
			return c, fmt.Errorf("decoding config file: %w", err)
		}

		if err := dataFile.Close(); err != nil {
			logrus.WithError(err).WithField("path", fn).Debug("closing config file (leaked fd)")
		}
	}

	return c, nil
}
