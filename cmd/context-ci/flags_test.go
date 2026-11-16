package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestFlagsWorkOnEitherSideOfThePositionals: the CLI reference writes `diff <base> <head>
// [--format ...]`, and the flag package stops at the first positional, so the documented order
// failed with "need a base and a head manifest". Every order has to produce the same report.
func TestFlagsWorkOnEitherSideOfThePositionals(t *testing.T) {
	base := manifestFor(t, repo(t, "gpt-x"), "base.json")
	head := manifestFor(t, repo(t, "gpt-y"), "head.json")

	orders := map[string][]string{
		"flags first":          {"diff", "--format", "json", base, head},
		"flags last":           {"diff", base, head, "--format", "json"},
		"flags between":        {"diff", base, "--format", "json", head},
		"the = form, last":     {"diff", base, head, "--format=json"},
		"single dash, last":    {"diff", base, head, "-format", "json"},
		"positionals after --": {"diff", "--format", "json", "--", base, head},
	}
	var first string
	for name, args := range orders {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != ExitOK {
				t.Fatalf("run(%v) exit = %d, stderr: %s", args, code, stderr.String())
			}
			var result struct {
				Entries []struct {
					Class string `json:"class"`
				} `json:"entries"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("--format json was not honoured, the output is not JSON: %v\n%s", err, stdout.String())
			}
			if len(result.Entries) == 0 || result.Entries[0].Class != "model.id" {
				t.Fatalf("entries = %+v", result.Entries)
			}
			if first == "" {
				first = stdout.String()
			} else if stdout.String() != first {
				t.Fatalf("this order produced a different report:\n%s\nwant:\n%s", stdout.String(), first)
			}
		})
	}
}

// TestInterleavedFlagsDoNotHideAStrayArgument: resuming after a positional must not turn a verb
// that takes none into one that accepts anything.
func TestInterleavedFlagsDoNotHideAStrayArgument(t *testing.T) {
	root := repo(t, "gpt-x")
	base := manifestFor(t, root, "base.json")
	cases := map[string][]string{
		"manifest": {"manifest", "stray", "--root", root, "--commit", "abc"},
		"check":    {"check", "stray", "--root", root, "--baseline", base, "--commit", "abc"},
		"init":     {"init", "stray", "--root", root, "--force"},
		"diff":     {"diff", base, base, "third", "--format", "json"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != ExitUsage {
				t.Fatalf("run(%v) exit = %d, want %d\n%s", args, code, ExitUsage, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("a usage error still wrote to stdout:\n%s", stdout.String())
			}
		})
	}
}

// TestHelpIsNotAnError: asking a verb for its flags exits 0 and prints them on stdout, so
// `context-ci check -h` can be piped. A flag that does not exist is still exit 2, on stderr.
func TestHelpIsNotAnError(t *testing.T) {
	for _, verb := range []string{"init", "manifest", "diff", "check", "attribute", "module-manifest", "serve"} {
		for _, flag := range []string{"-h", "--help", "-help"} {
			t.Run(verb+" "+flag, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				if code := run([]string{verb, flag}, &stdout, &stderr); code != ExitOK {
					t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stderr.String())
				}
				if !strings.HasPrefix(stdout.String(), "usage: context-ci "+verb) {
					t.Fatalf("help is not on stdout:\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
				}
				if stderr.Len() != 0 {
					t.Fatalf("help wrote to stderr: %s", stderr.String())
				}
			})
		}
		t.Run(verb+" with a flag that does not exist", func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run([]string{verb, "--no-such-flag"}, &stdout, &stderr); code != ExitUsage {
				t.Fatalf("exit = %d, want %d", code, ExitUsage)
			}
			if !strings.Contains(stderr.String(), "no-such-flag") || !strings.Contains(stderr.String(), "usage: context-ci "+verb) {
				t.Fatalf("the error does not name the flag and the usage: %s", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("a usage error wrote to stdout: %s", stdout.String())
			}
		})
	}

	// Help after a positional is still help.
	var stdout, stderr bytes.Buffer
	if code := run([]string{"diff", "a.json", "b.json", "-h"}, &stdout, &stderr); code != ExitOK {
		t.Fatalf("diff a.json b.json -h exit = %d, want %d", code, ExitOK)
	}
	// The flags are listed, with the note that makes the documented order legal.
	if !strings.Contains(stdout.String(), "-format") || !strings.Contains(stdout.String(), "before or after") {
		t.Fatalf("the help does not list the flags:\n%s", stdout.String())
	}
}
