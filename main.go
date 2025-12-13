package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var (
	// Version information (set via ldflags during build)
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Parse command line flags
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Show version and exit
	if *showVersion {
		fmt.Printf("DimanDocs %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Get config file from command line args
	configFile := ""
	if flag.NArg() > 0 {
		configFile = flag.Arg(0)
	}

	// Create and initialize application
	app := NewApp()
	if err := app.Initialize(configFile); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Start the server
	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}