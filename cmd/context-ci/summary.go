package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sigilary/context-ci/internal/diff"
)

// appendSummary appends a Markdown report to a file. On GitHub the file is $GITHUB_STEP_SUMMARY,
// and the report appears on the run's page: the place a reviewer lands after clicking a red check,
// and the only place the changed text can be shown, because an annotation is one line.
//
// It appends because the file is shared by every step of a job. Nothing is fetched and nothing is
// posted: this writes a local file, and the runner does the rest.
func appendSummary(path string, r *diff.Result, gateFailed bool, failOn string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(markdownSummary(r, gateFailed, failOn)); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func markdownSummary(r *diff.Result, gateFailed bool, failOn string) string {
	var b strings.Builder
	if r.Equal() {
		fmt.Fprintf(&b, "### Context CI: no change in context\n\nDigest `%s`.\n", r.HeadDigest)
		return b.String()
	}

	verdict := "the gate passes"
	if gateFailed {
		verdict = "the gate fails"
	}
	fmt.Fprintf(&b, "### Context CI: %d change(s) in context, and %s\n\n", len(r.Entries), verdict)
	fmt.Fprintf(&b, "The gate fails on `%s`. ", failOn)
	if acknowledged := countAcknowledged(r); acknowledged > 0 {
		fmt.Fprintf(&b, "%d change(s) were acknowledged for this run and do not count. ", acknowledged)
	}
	b.WriteString("Every row is a change between the baseline and this checkout.\n\n")

	b.WriteString("| Severity | Class | Change | Where |\n|---|---|---|---|\n")
	for _, e := range r.Entries {
		severity := string(e.Severity)
		if e.Acknowledged {
			severity += " (acknowledged)"
		}
		change := e.Summary
		if e.Before != "" || e.After != "" {
			change += fmt.Sprintf(": `%s` to `%s`", orDash(e.Before), orDash(e.After))
		}
		if e.Detail != "" {
			change += ". " + e.Detail
		}
		where := ""
		if at := location(e); at != "" {
			where = "`" + at + "`"
		}
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", severity, e.Class, cell(change), where)
	}

	for _, text := range r.Texts {
		// The fence has to be longer than any run of backticks inside it, or a prompt that
		// contains a code block would end the fence early and render the rest as Markdown.
		fence := strings.Repeat("`", longestBacktickRun(text.Diff)+1)
		if len(fence) < 3 {
			fence = "```"
		}
		fmt.Fprintf(&b, "\n<details><summary>%s</summary>\n\n%sdiff\n%s\n%s\n", escapeHTML(text.Path), fence, text.Diff, fence)
		if text.Omitted > 0 {
			fmt.Fprintf(&b, "\n%d more line(s) are not shown.\n", text.Omitted)
		}
		b.WriteString("\n</details>\n")
	}

	fmt.Fprintf(&b, "\nDigest `%s` to `%s`.\n", short(r.BaseDigest), short(r.HeadDigest))
	return b.String()
}

func countAcknowledged(r *diff.Result) int {
	n := 0
	for _, e := range r.Entries {
		if e.Acknowledged {
			n++
		}
	}
	return n
}

// cell makes a value safe inside a table cell: a pipe would start a new column and a newline would
// end the row.
func cell(s string) string {
	return strings.NewReplacer("|", `\|`, "\r", " ", "\n", " ", "<", "&lt;").Replace(s)
}

func escapeHTML(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func longestBacktickRun(s string) int {
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
			continue
		}
		run = 0
	}
	return longest
}
