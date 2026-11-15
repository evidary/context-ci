package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/evidary/context-ci/internal/config"
)

// defaultPrompt is written when the checkout holds no file the probe list recognises. The
// prompt source is required by the configuration validator, so init cannot leave it out the way
// it leaves out a source it did not find; it names the documented path and says on stderr that
// the path does not exist yet.
const defaultPrompt = "prompts/support.md"

// optionalSource is a source init can name, with the documented path it writes as a comment when
// the checkout has no file to name. The examples are the ones the CLI reference's example
// configuration uses, so the generated file and the documentation do not disagree.
type optionalSource struct {
	kind    string
	example string
	probes  []string
}

// optionalSources are rendered in this order, which is the order the CLI reference uses.
var optionalSources = []optionalSource{
	{
		kind:    config.SourceTools,
		example: "config/tools.yaml",
		probes:  []string{"config/tools.yaml", "config/tools.yml", "tools.yaml", "tools.yml"},
	},
	{
		kind:    config.SourceRetrieval,
		example: "config/retrieval.yaml",
		probes:  []string{"config/retrieval.yaml", "config/retrieval.yml", "retrieval.yaml", "retrieval.yml"},
	},
	{
		kind:    config.SourceGuardrails,
		example: "config/guardrails.yaml",
		probes:  []string{"config/guardrails.yaml", "config/guardrails.yml", "guardrails.yaml", "guardrails.yml"},
	},
	{
		kind:    config.SourceRendered,
		example: "build/rendered.txt",
		probes:  []string{"build/rendered.txt", "build/rendered.md"},
	},
}

// promptProbes are the places init looks for a prompt template, in preference order. The list is
// short and literal on purpose: a scanner that searched a whole tree would sometimes name the
// wrong file, and a configuration that hashes the wrong prompt is worse than one that asks the
// team to create the path it names.
var promptProbes = []string{
	"prompts/support.md",
	"prompts/system.md",
	"prompts/system-prompt.md",
	"prompts/prompt.md",
	"prompt.md",
	"PROMPT.md",
}

// runInit writes a commented starter configuration for a checkout.
//
// It exists because the alternative is a team copying YAML out of a documentation example, and a
// configuration typo is a gate that looks on while being off. The file is parsed by the same
// strict loader check uses before a byte of it reaches the disk, so init cannot produce a file
// the next command rejects.
//
// Only one thing is discovered: which conventional source files this checkout already has. The
// model block is a placeholder and says so, because a repository does not record which model it
// calls in a place this tool may trust, and a guess written as fact would gate on a model the
// team does not use.
func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		root       = fs.String("root", ".", "repository root the configuration describes")
		configPath = fs.String("config", "", "where to write it (default "+config.DefaultFile+" under --root)")
		force      = fs.Bool("force", false, "replace a configuration file that already exists")
	)
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "init: unexpected argument %q\n", fs.Arg(0))
		return ExitUsage
	}

	path := *configPath
	if path == "" {
		path = filepath.Join(*root, config.DefaultFile)
	}
	// An existing file can be the only record of what a team gates on, so it is never replaced
	// without being asked. --force is the asking, and a refusal is a usage error rather than a
	// gate failure: nothing was compared, so nothing regressed.
	if _, err := os.Stat(path); err == nil {
		if !*force {
			fmt.Fprintf(stderr, "init: %s already exists; pass --force to replace it\n", path)
			return ExitUsage
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "init: cannot tell whether %s exists: %v\n", path, err)
		return ExitUsage
	}

	sources, missing := discoverSources(*root)
	body := starterConfig(sources, missing)
	if _, err := config.Parse(body); err != nil {
		fmt.Fprintf(stderr, "init: the generated configuration does not validate, which is a bug in context-ci: %v\n", err)
		return ExitUsage
	}
	// The default location is the one every other verb reads, so the file needs no argument on
	// any later command.
	if err := os.WriteFile(path, body, 0o644); err != nil {
		fmt.Fprintf(stderr, "init: write %s: %v\n", path, err)
		return ExitUsage
	}

	fmt.Fprintf(stdout, "wrote %s\n", path)
	for _, rel := range missing {
		fmt.Fprintf(stderr, "context-ci: %s does not exist yet, so 'context-ci manifest' will fail until it does: "+
			"create it, or point capture.sources.prompt at the file that holds the prompt\n", rel)
	}
	if absent := absentKinds(sources); len(absent) > 0 {
		fmt.Fprintf(stderr, "context-ci: no %s source found in this checkout, so that line is commented out\n",
			strings.Join(absent, ", "))
	}
	fmt.Fprintf(stderr, "context-ci: model.provider and model.id are placeholders; edit them before the first check\n")
	return ExitOK
}

// discoverSources names a source only when the checkout already has the file, because a
// configuration pointing at a path nobody created fails the first manifest for a reason the team
// then has to discover. The prompt is the exception: the configuration is invalid without it, so
// a default path is written and reported as missing.
func discoverSources(root string) (map[string]string, []string) {
	sources := make(map[string]string, 1+len(optionalSources))
	var missing []string
	if found := firstExisting(root, promptProbes); found != "" {
		sources[config.SourcePrompt] = found
	} else {
		sources[config.SourcePrompt] = defaultPrompt
		missing = append(missing, defaultPrompt)
	}
	for _, optional := range optionalSources {
		if found := firstExisting(root, optional.probes); found != "" {
			sources[optional.kind] = found
		}
	}
	return sources, missing
}

// absentKinds lists the optional sources this checkout has no file for, in the order they are
// written, so the message names them the way the file does.
func absentKinds(sources map[string]string) []string {
	var absent []string
	for _, optional := range optionalSources {
		if _, ok := sources[optional.kind]; !ok {
			absent = append(absent, optional.kind)
		}
	}
	return absent
}

// firstExisting returns the first candidate that is a regular file under root, or "". A directory
// with the right name is not a source: hashing it would fail, and naming it would produce the
// configuration init exists to avoid.
func firstExisting(root string, candidates []string) string {
	for _, rel := range candidates {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil && info.Mode().IsRegular() {
			return rel
		}
	}
	return ""
}

// starterConfig renders the file. The comments carry the parts of the contract a team cannot
// guess: that a source path must exist and must stay inside the repository, that a sampling value
// is hashed as literal text, and that an unknown key is rejected rather than ignored.
func starterConfig(sources map[string]string, missing []string) []byte {
	var b strings.Builder
	b.WriteString(starterHead)
	fmt.Fprintf(&b, "    prompt: %s\n", sources[config.SourcePrompt])
	if len(missing) > 0 {
		// The file says it about itself as well as init saying it on stderr: whoever opens the
		// file tomorrow is the reader this sentence is for.
		fmt.Fprintf(&b, "    # %s does not exist in this checkout yet: create it, or point this line at the\n"+
			"    # file that holds the prompt. 'context-ci manifest' fails until one of those is done.\n", missing[0])
	}
	b.WriteString(starterOptionalComment)
	for _, optional := range optionalSources {
		if rel, ok := sources[optional.kind]; ok {
			fmt.Fprintf(&b, "    %s: %s\n", optional.kind, rel)
			continue
		}
		fmt.Fprintf(&b, "    # %s: %s\n", optional.kind, optional.example)
	}
	b.WriteString(starterTail)
	return []byte(b.String())
}

const starterHead = `# .context-ci.yaml - written by 'context-ci init'.
#
# 'context-ci manifest' hashes every file named under capture.sources and records the values
# below beside the hashes, so this file is part of the context: commit it with the change that
# edits it.
#
# Every value below is a starting point, not a measurement. Edit model.provider and model.id
# before the first real check, because a manifest naming a model the team does not call gates on
# a change nobody made.

version: 1

capture:
  # asserted is the only path this CLI accepts: the context is assembled from files in this
  # repository. instrumented needs an SDK capture and observed needs a recorded sample, and
  # neither can be claimed by a configuration file.
  path: asserted
  sources:
    # Required: a manifest with no prompt hash proves nothing.
    # Every source path is relative to the repository root and may not leave it.
`

const starterOptionalComment = `    # Optional. A line here must name a file that exists; leaving one out keeps that part of
    # the context out of the manifest. A commented line names a path this checkout does not have.
`

const starterTail = `
model:
  # openai | anthropic | google | other
  provider: openai
  id: gpt-x
  # The stable name id resolves to. Uncomment it so an alias moving is a diff of its own instead
  # of a silent change of behaviour.
  # alias_of: gpt-x-latest
  params:
    # Quoted, because the literal text is hashed: 0 and 0.0 are the same setting and different
    # bytes, and the digest must not depend on how a parser spells a number.
    temperature: "0"
    max_tokens: 1024

# retrieval and policy stay commented until the values are known: a value invented here would be
# recorded as configuration the team never chose.
# retrieval:
#   index_id: kb-prod
#   embedding_model: embed-3
#   embedding_model_version: "2026-05"
#   chunker: recursive-800
#   top_k: 5
# policy:
#   version: "2026.11"

severity:
  # Override any class: fail | warn | ignore. Run context-ci classes for the list, and note that
  # an unknown key is rejected rather than ignored.
  # model.id: fail
  # retrieval.top_k: warn

gate:
  # fail_on: fail | warn | never. fail is the default and the protective one.
  fail_on: fail
  # Optional, and used only to turn a token delta into a dollar projection.
  # calls_per_day: 20000
  # input_usd_per_mtok: 3
`
