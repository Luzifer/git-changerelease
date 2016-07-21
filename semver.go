package main

import (
	"errors"
	"strconv"
	"strings"
)

type semVerBump uint

const (
	semVerBumpUndecided semVerBump = iota
	semVerBumpPatch
	semVerBumpMinor
	semVerBumpMajor
)

type semVer struct {
	Major, Minor, Patch   int
	PreReleaseInformation string
	MetaData              string
}

func (s *semVer) String() string {
	v := []string{strings.Join([]string{
		strconv.Itoa(s.Major),
		strconv.Itoa(s.Minor),
		strconv.Itoa(s.Patch),
	}, ".")}

	if s.PreReleaseInformation != "" {
		v = append(v, "-"+s.PreReleaseInformation)
	}
	if s.MetaData != "" {
		v = append(v, "+"+s.MetaData)
	}

	return strings.Join(v, "")
}

func parseSemVer(version string) (*semVer, error) {
	var (
		s   semVer
		err error
	)
	version = strings.TrimLeft(version, "v") // Ensure the version is not prefixed like v0.1.0

	t := strings.SplitN(version, "+", 2)
	if len(t) == 2 {
		s.MetaData = t[1]
	}

	t = strings.SplitN(t[0], "-", 2)
	if len(t) == 2 {
		s.PreReleaseInformation = t[1]
	}

	elements := strings.Split(t[0], ".")
	if len(elements) != 3 {
		return nil, errors.New("Version does not match semantic versioning format")
	}

	s.Major, err = strconv.Atoi(elements[0])
	if err != nil {
		return nil, err
	}
	s.Minor, err = strconv.Atoi(elements[1])
	if err != nil {
		return nil, err
	}
	s.Patch, err = strconv.Atoi(elements[2])
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *semVer) Bump(bumpType semVerBump) {
	switch bumpType {
	case semVerBumpPatch:
		s.Patch += 1
	case semVerBumpMinor:
		s.Patch = 0
		s.Minor += 1
	case semVerBumpMajor:
		s.Patch = 0
		s.Minor = 0
		s.Major += 1
	}
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
		return semVerBumpUndecided, errors.New("Could not decide for any bump type!")
	}

	return bump, nil
}
