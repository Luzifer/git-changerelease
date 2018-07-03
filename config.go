package main

import (
	"errors"
	"os"

	yaml "gopkg.in/yaml.v2"
)

type configFile struct {
	DiableTagSigning     bool     `yaml:"disable_signed_tags"`
	MatchMajor           []string `yaml:"match_major"`
	MatchPatch           []string `yaml:"match_patch"`
	ReleaseCommitMessage string   `yaml:"release_commit_message"`
}

func loadConfig() (*configFile, error) {
	var err error

	if _, err = os.Stat(cfg.ConfigFile); err != nil {
		return nil, errors.New("Config file does not exist, use --create-config to create one")
	}

	c := &configFile{}
	if err = yaml.Unmarshal(MustAsset("assets/git_changerelease.yaml"), c); err != nil {
		return nil, err
	}

	dataFile, err := os.Open(cfg.ConfigFile)
	if err != nil {
		return nil, err
	}
	defer dataFile.Close()

	return c, yaml.NewDecoder(dataFile).Decode(c)
}
