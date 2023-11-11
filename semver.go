package main

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
)

type semVerBump uint

const (
	semVerBumpUndecided semVerBump = iota
	semVerBumpPatch
	semVerBumpMinor
	semVerBumpMajor
)

type semVer struct {
	*semver.Version
}

func (s *semVer) SetMetadata(metadata string) error {
	nv, err := s.Version.SetMetadata(metadata)
	if err != nil {
		return fmt.Errorf("setting metadata: %w", err)
	}

	s.Version = &nv
	return nil
}

func (s *semVer) SetPrerelease(prerelease string) error {
	nv, err := s.Version.SetPrerelease(prerelease)
	if err != nil {
		return fmt.Errorf("setting prerelease: %w", err)
	}

	s.Version = &nv
	return nil
}

func parseSemVer(version string) (*semVer, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("parsing semver: %w", err)
	}
	return &semVer{v}, nil
}

func (s *semVer) Bump(bumpType semVerBump) {
	var nv semver.Version

	switch bumpType {
	case semVerBumpPatch:
		nv = s.Version.IncPatch()

	case semVerBumpMinor:
		nv = s.Version.IncMinor()

	case semVerBumpMajor:
		nv = s.Version.IncMajor()
	}

	s.Version = &nv
}

func selectBumpType(logs []commit) (semVerBump, error) {
	bump := semVerBumpUndecided

	for _, l := range logs {
		if l.BumpType > bump {
			bump = l.BumpType
		}
	}

	if bump == semVerBumpUndecided {
		// Impossible to reach
		return semVerBumpUndecided, errors.New("could not decide for any bump type")
	}

	return bump, nil
}
