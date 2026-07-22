package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/li-daqian/local-config-sync/internal/releasemanifest"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("release-manifest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("file", "", "path to the release manifest YAML")
	releaseTag := flags.String("tag", "", "release tag that must match releaseId")
	jsonOutput := flags.Bool("json", false, "write the normalized manifest as JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "release-manifest: positional arguments are not supported")
		return 2
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "release-manifest: --file is required")
		return 2
	}
	if *releaseTag == "" {
		fmt.Fprintln(stderr, "release-manifest: --tag is required")
		return 2
	}

	manifest, err := releasemanifest.Load(*manifestPath, *releaseTag)
	if err != nil {
		fmt.Fprintf(stderr, "release-manifest: %v\n", err)
		return 1
	}

	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(manifest); err != nil {
			fmt.Fprintf(stderr, "release-manifest: encode JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Release manifest %s is valid (%d artifact(s)).\n", manifest.ReleaseID, len(manifest.Artifacts))
	return 0
}
