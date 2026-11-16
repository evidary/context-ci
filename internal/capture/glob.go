package capture

import (
	"path"
	"strings"
)

// MatchPattern reports whether a slash-separated path matches a pattern.
//
// `*`, `?` and `[...]` match within one path segment, as path.Match defines them. A segment that
// is exactly `**` matches any number of directories, including none, so `**/AGENTS.md` matches the
// file at the root as well as one three directories down. A pattern with no wildcard also matches
// everything below it, so `prompts` and `prompts/` both mean the directory's whole content.
//
// Matching is case-sensitive on every platform. A manifest built on Windows and one built on
// Linux have to list the same files, and only one of those filesystems would otherwise agree that
// agents.md is AGENTS.md.
func MatchPattern(pattern, name string) bool {
	pattern = strings.TrimSuffix(pattern, "/")
	segments := strings.Split(pattern, "/")
	parts := strings.Split(name, "/")
	if matchSegments(segments, parts) {
		return true
	}
	if !strings.ContainsAny(pattern, "*?[") {
		return matchSegments(append(segments, "**"), parts)
	}
	return false
}

func matchSegments(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			rest := pattern[1:]
			if len(rest) == 0 {
				// A trailing ** stands for at least one more segment: a directory is not a file.
				return len(name) > 0
			}
			for skip := 0; skip <= len(name); skip++ {
				if matchSegments(rest, name[skip:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, err := path.Match(pattern[0], name[0]); err != nil || !ok {
			return false
		}
		pattern, name = pattern[1:], name[1:]
	}
	return len(name) == 0
}

// literalPath reports whether a pattern names exactly one path, so it can be read without a
// listing and reported as missing when it is.
func literalPath(pattern string) bool {
	return !strings.ContainsAny(pattern, "*?[") && !strings.HasSuffix(pattern, "/")
}
