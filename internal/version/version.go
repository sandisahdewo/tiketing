package version

import (
	"strings"

	"golang.org/x/mod/semver"
)

func Normalize(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

func Valid(v string) bool {
	return semver.IsValid(Normalize(v))
}

func Newer(candidate, current string) bool {
	c, cur := Normalize(candidate), Normalize(current)
	if !semver.IsValid(c) || !semver.IsValid(cur) {
		return false
	}
	return semver.Compare(c, cur) > 0
}

func Compatible(a, b string) bool {
	na, nb := Normalize(a), Normalize(b)
	if !semver.IsValid(na) || !semver.IsValid(nb) {
		return false
	}
	return semver.MajorMinor(na) == semver.MajorMinor(nb)
}
