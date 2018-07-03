package main

import (
	"reflect"
	"testing"
)

func TestSemVerParseValid(t *testing.T) {
	tests := map[string]semVer{
		"1.9.0":                      {Major: 1, Minor: 9, Patch: 0, PreReleaseInformation: "", MetaData: ""},
		"4.9.0":                      {Major: 4, Minor: 9, Patch: 0, PreReleaseInformation: "", MetaData: ""},
		"1068.6.0":                   {Major: 1068, Minor: 6, Patch: 0, PreReleaseInformation: "", MetaData: ""},
		"1.0.0-alpha":                {Major: 1, Minor: 0, Patch: 0, PreReleaseInformation: "alpha", MetaData: ""},
		"1.0.0-alpha.1":              {Major: 1, Minor: 0, Patch: 0, PreReleaseInformation: "alpha.1", MetaData: ""},
		"1.0.0-0.3.7":                {Major: 1, Minor: 0, Patch: 0, PreReleaseInformation: "0.3.7", MetaData: ""},
		"1.0.0-x.7.z.92":             {Major: 1, Minor: 0, Patch: 0, PreReleaseInformation: "x.7.z.92", MetaData: ""},
		"1.0.0-alpha+001":            {Major: 1, Minor: 0, Patch: 0, PreReleaseInformation: "alpha", MetaData: "001"},
		"1.0.0+20130313144700":       {Major: 1, Minor: 0, Patch: 0, PreReleaseInformation: "", MetaData: "20130313144700"},
		"1.0.0-beta+exp.sha.5114f85": {Major: 1, Minor: 0, Patch: 0, PreReleaseInformation: "beta", MetaData: "exp.sha.5114f85"},
	}

	for version, exp := range tests {
		s, e := parseSemVer(version)
		if e != nil {
			t.Errorf("Parse of version '%s' failed: %s", version, e)
		}
		if !reflect.DeepEqual(exp, *s) {
			t.Errorf("Parse of version '%s' (%#v) did not match expectation: %#v", version, exp, s)
		}
	}
}
