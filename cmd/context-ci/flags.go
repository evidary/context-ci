package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// parseArgs parses one verb's arguments and returns its positional arguments.
//
// Two things the flag package does by default are wrong for this tool, and both are fixed here so
// that every verb behaves the same way:
//
//   - It stops at the first positional, so `diff a.json b.json --format sarif` — the order the CLI
//     reference documents — left --format unparsed and failed with "need a base and a head
//     manifest". Parsing resumes after each positional, so a flag works wherever it is written.
//     A bare `--` still ends the flags, for a file whose name starts with a dash.
//   - It reports -h as an error. Asking for help is not a usage error: the flags are printed on
//     stdout and the exit code is 0, so `context-ci check -h` can be piped and scripted.
//
// When ok is false the caller returns code without doing anything else.
func parseArgs(fs *flag.FlagSet, synopsis string, args []string, stdout, stderr io.Writer) (positional []string, code int, ok bool) {
	fs.SetOutput(stderr)
	// The flag package would print its own "Usage of <verb>:" block; the synopsis says more, and
	// where it is printed depends on whether help was asked for or a mistake was made.
	fs.Usage = func() {}
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				printVerbUsage(fs, synopsis, stdout)
				return nil, ExitOK, false
			}
			// The flag package has already written what was wrong to stderr.
			printVerbUsage(fs, synopsis, stderr)
			return nil, ExitUsage, false
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positional, ExitOK, true
		}
		if consumed := len(args) - len(rest); consumed > 0 && args[consumed-1] == "--" {
			return append(positional, rest...), ExitOK, true
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

func printVerbUsage(fs *flag.FlagSet, synopsis string, w io.Writer) {
	fmt.Fprintf(w, "usage: %s\n", synopsis)
	hasFlags := false
	fs.VisitAll(func(*flag.Flag) { hasFlags = true })
	if !hasFlags {
		return
	}
	fmt.Fprintln(w, "\nflags (may be written before or after any arguments):")
	fs.SetOutput(w)
	fs.PrintDefaults()
}
