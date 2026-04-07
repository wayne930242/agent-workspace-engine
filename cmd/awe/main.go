package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/wayne930242/agent-workspace-engine/internal/exporter"
	"github.com/wayne930242/agent-workspace-engine/internal/planner"
	"github.com/wayne930242/agent-workspace-engine/internal/workspacefile"
)

func main() {
	var (
		path       string
		outDir     string
		write      bool
		strictAuth bool
	)
	flag.StringVar(&path, "file", "Workspacefile", "path to Workspacefile")
	flag.StringVar(&outDir, "out", "build/workspace", "output directory for generated bundle")
	flag.BoolVar(&write, "write", false, "write manifest and runtime exports to the output directory")
	flag.BoolVar(&strictAuth, "strict-auth", false, "fail build when declared auth strategies are unavailable")
	flag.Parse()

	doc, err := workspacefile.ParseFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse Workspacefile: %v\n", err)
		os.Exit(1)
	}

	plan, err := planner.Build(doc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build plan: %v\n", err)
		os.Exit(1)
	}

	if write {
		if err := exporter.WriteBundleWithOptions(outDir, plan, exporter.Options{
			StrictAuth: strictAuth,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "write bundle: %v\n", err)
			os.Exit(1)
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(plan); err != nil {
		fmt.Fprintf(os.Stderr, "encode plan: %v\n", err)
		os.Exit(1)
	}
}
