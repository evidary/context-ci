package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// TestUsageNamesEveryDispatchedVerb keeps `--help` from lying by omission. `attribute` shipped in
// 0.6.0 and the help screen never mentioned it, so the only way to learn the verb existed was the
// changelog or the source. The check reads the dispatcher rather than a hand-kept list, because a
// list is the thing that was out of date.
func TestUsageNamesEveryDispatchedVerb(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	words := map[string]bool{}
	for _, word := range strings.Fields(usage) {
		words[strings.Trim(word, "[]<>(),.:`")] = true
	}
	for _, verb := range dispatchedVerbs(t, file) {
		// A long form such as --version is an alias of the verb it names, so it is satisfied by
		// the verb being listed.
		name := strings.TrimPrefix(verb, "--")
		if helpAliases[name] {
			continue
		}
		if !words[name] {
			t.Errorf("verb %q is dispatched but the usage text never names it", verb)
		}
	}
}

// helpAliases are the conventional ways to ask for the help screen itself; naming them in the help
// screen would be noise, and the last line of the usage text already says how to ask.
var helpAliases = map[string]bool{"help": true, "-h": true}

// dispatchedVerbs returns the string literals of the switch inside run, which is the verb
// dispatcher. Sub-switches live in their own functions, so they do not leak in here.
func dispatchedVerbs(t *testing.T, file *ast.File) []string {
	t.Helper()
	var verbs []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "run" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			stmt, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			for _, item := range stmt.Body.List {
				clause, ok := item.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range clause.List {
					if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if verb, err := strconv.Unquote(lit.Value); err == nil {
							verbs = append(verbs, verb)
						}
					}
				}
			}
			return false
		})
	}
	if len(verbs) == 0 {
		t.Fatal("found no verbs in run's dispatcher, so this check proved nothing")
	}
	return verbs
}
