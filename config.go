package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

type configFile struct {
	DiableTagSigning     bool     `yaml:"disable_signed_tags"`
	MatchMajor           []string `yaml:"match_major"`
	MatchPatch           []string `yaml:"match_patch"`
	ReleaseCommitMessage string   `yaml:"release_commit_message"`
	IgnoreMessages       []string `yaml:"ignore_messages"`
}

func loadConfig() (*configFile, error) {
	var err error

	if _, err = os.Stat(cfg.ConfigFile); err != nil {
		return nil, errors.New("config file does not exist, use --create-config to create one")
	}

	c := &configFile{}
	if err = yaml.Unmarshal(mustAsset("assets/git_changerelease.yaml"), c); err != nil {
		return nil, fmt.Errorf("unmarshalling default config: %w", err)
	}

	dataFile, err := os.Open(cfg.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("opening config file: %w", err)
	}
	defer func() {
		if err := dataFile.Close(); err != nil {
			logrus.WithError(err).Debug("closing config file (leaked fd)")
		}
	}()

	if err = yaml.NewDecoder(dataFile).Decode(c); err != nil {
		return c, fmt.Errorf("decoding config file: %w", err)
	}

	return c, nil
}
