package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/evidary/context-ci/internal/modulemanifest"
)

// runModuleManifest prints this module's module-manifest.v1.json.
//
// It is deliberately not called `manifest`. This product already has a `manifest` verb, and it means something
// else: the context manifest, context-manifest.json, for a checkout. Two documents with one name would be a
// trap for whoever scripts against it, so the platform's discovery document gets the name of the contract it
// emits — and that name is the same in every tool that speaks the contract.
func runModuleManifest(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("module-manifest", flag.ContinueOnError)
	positional, code, ok := parseArgs(fs, "context-ci module-manifest", args, stdout, stderr)
	if !ok {
		return code
	}
	if len(positional) != 0 {
		fmt.Fprintf(stderr, "module-manifest: unexpected argument %q\n", positional[0])
		return ExitUsage
	}
	built, err := modulemanifest.Build(version)
	if err != nil {
		// Not a usage error: the command was used correctly and the build has no released identity.
		fmt.Fprintf(stderr, "module-manifest: %v\n", err)
		return ExitManifest
	}
	if err := encodeJSON(stdout, built); err != nil {
		fmt.Fprintf(stderr, "module-manifest: %v\n", err)
		return ExitUsage
	}
	return ExitOK
}

// runServe answers the two probes a platform asks a module it hosts.
//
// `module-manifest.v1.json` requires `health_path`, and a manifest naming an endpoint nobody serves is a claim
// a probe disproves within seconds — so the endpoint is real, and it is the minimum.
//
// Readiness says `stateless: true`, which is the honest answer rather than a manufactured dependency. This
// product reads a repository when a check is asked for; a repository that fails a gate is the product working,
// not the product being unhealthy.
func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8099", "address to listen on")
	positional, code, ok := parseArgs(fs, "context-ci serve [flags]", args, stdout, stderr)
	if !ok {
		return code
	}
	if len(positional) != 0 {
		fmt.Fprintf(stderr, "serve: unexpected argument %q\n", positional[0])
		return ExitUsage
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return ExitUsage
	}
	server := &http.Server{Handler: probeServer(version), ReadHeaderTimeout: 10 * time.Second}
	fmt.Fprintf(stdout, "listening on http://%s%s\n", listener.Addr(), modulemanifest.HealthPath)
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return ExitManifest
	}
	return ExitOK
}

func probeServer(buildVersion string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(modulemanifest.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		writeProbe(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"module_id": modulemanifest.ModuleID,
			"version":   buildVersion,
		})
	})
	mux.HandleFunc(modulemanifest.ReadyPath, func(w http.ResponseWriter, r *http.Request) {
		writeProbe(w, http.StatusOK, map[string]any{
			"status":    "ready",
			"module_id": modulemanifest.ModuleID,
			"version":   buildVersion,
			"stateless": true,
		})
	})
	return mux
}

func writeProbe(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = w.Write(append(body, '\n'))
}
