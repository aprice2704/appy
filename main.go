package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const AppVersion = "v1.8.17"

func watchSelfForReload() {
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Hot reload watcher disabled: cannot determine executable path: %v", err)
		return
	}
	stat, err := os.Stat(execPath)
	if err != nil {
		log.Printf("Hot reload watcher disabled: cannot stat executable: %v", err)
		return
	}
	initialModTime := stat.ModTime()

	for {
		time.Sleep(1 * time.Second)
		stat, err := os.Stat(execPath)
		if err == nil && stat.ModTime().After(initialModTime) {
			log.Printf("Binary updated (mod time changed). Triggering hot reload (Exit 42)...")
			os.Exit(42)
		}
	}
}

func main() {
	port := flag.String("port", "8085", "Port to run the appy server on")
	largeFileLines := flag.Int("large-file-lines", 350, "Line threshold to inject 'split file' warnings into txtar bundles")
	buildSets := flag.String("build-sets", "", "Comma-separated list of sets to build immediately and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Appy %s - The Stateful Patch Console\n\n", AppVersion)
		fmt.Fprintf(os.Stderr, "Usage:\n  appy [flags]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n  appy -port 8080\n  appy -build-sets=\"core,frontend\"\n")
	}
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	if *buildSets != "" {
		sets := getSets(cwd)
		for _, setName := range strings.Split(*buildSets, ",") {
			setName = strings.TrimSpace(setName)
			if payload, ok := sets[setName]; ok {
				log.Printf("Building set: %s", setName)
				b, count, err := generateTxtar(cwd, payload, *largeFileLines)
				if err != nil {
					log.Fatalf("Failed to build set %s: %v", setName, err)
				}
				fname := payload.FileName
				if fname == "" {
					fname = fmt.Sprintf("appy_bundle_%s.txtar", setName)
				}
				if err := os.WriteFile(filepath.Join(cwd, fname), b, 0644); err != nil {
					log.Fatalf("WriteFile failed for %s: %v", fname, err)
				}
				log.Printf("Successfully built %s with %d files.", fname, count)
			} else {
				log.Printf("Warning: Set %q not found in .appy_sets.json", setName)
			}
		}
		os.Exit(0)
	}

	go watchSelfForReload()

	fmt.Printf("Appy %s on http://localhost:%s\n", AppVersion, *port)
	log.Fatal(http.ListenAndServe(":"+*port, newServer(cwd, *largeFileLines)))
}
