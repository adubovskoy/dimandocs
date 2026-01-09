package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"dimandocs/mcp"
)

var (
	// Version information (set via ldflags during build)
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Check for subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "index":
			runIndexCommand(os.Args[2:])
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	// Parse command line flags for main command
	showVersion := flag.Bool("version", false, "Show version information")
	mcpMode := flag.Bool("mcp", false, "Run in MCP server mode (stdio transport)")
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

	// Initialize embedding manager if enabled
	var embedManager *EmbeddingManager
	if app.Config.Embeddings.Enabled {
		var err error
		embedManager, err = NewEmbeddingManager(app.Config.Embeddings)
		if err != nil {
			log.Fatalf("Failed to initialize embedding manager: %v", err)
		}
		defer embedManager.Close()

		// Set embedding manager on app for vector search in web interface
		app.EmbeddingManager = embedManager

		// Index all documents
		ctx := context.Background()
		for _, doc := range app.Documents {
			if err := embedManager.IndexDocument(ctx, doc, false); err != nil {
				log.Printf("Warning: failed to index document %s: %v", doc.RelPath, err)
			}
		}
		log.Printf("Embedding indexing complete")
	}

	// MCP mode - run as MCP server
	if *mcpMode {
		if !app.Config.Embeddings.Enabled {
			log.Fatal("MCP mode requires embeddings to be enabled in config")
		}

		docProvider := NewAppDocumentProvider(app)

		mcpServer, err := mcp.NewServer(mcp.Config{
			Name:         "dimandocs",
			Version:      Version,
			VectorStore:  embedManager.GetVectorStore(),
			EmbedService: embedManager.GetEmbedService(),
			DocProvider:  docProvider,
		})
		if err != nil {
			log.Fatalf("Failed to create MCP server: %v", err)
		}

		log.Println("Starting MCP server over stdio...")
		if err := mcpServer.ServeStdio(); err != nil {
			log.Fatalf("MCP server error: %v", err)
		}
		return
	}

	// Normal mode - start HTTP server
	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// runIndexCommand handles the "index" subcommand
func runIndexCommand(args []string) {
	indexFlags := flag.NewFlagSet("index", flag.ExitOnError)
	force := indexFlags.Bool("force", false, "Force re-indexing of all documents, ignoring cache")
	indexFlags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dimandocs index [options] [config_file]\n\n")
		fmt.Fprintf(os.Stderr, "Index documents for semantic search.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		indexFlags.PrintDefaults()
	}
	indexFlags.Parse(args)

	// Get config file
	configFile := ""
	if indexFlags.NArg() > 0 {
		configFile = indexFlags.Arg(0)
	}

	// Create and initialize application
	app := NewApp()
	if err := app.Initialize(configFile); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Check if embeddings are enabled
	if !app.Config.Embeddings.Enabled {
		log.Fatal("Embeddings are not enabled in config. Add 'embeddings' section to dimandocs.json")
	}

	// Initialize embedding manager
	embedManager, err := NewEmbeddingManager(app.Config.Embeddings)
	if err != nil {
		log.Fatalf("Failed to initialize embedding manager: %v", err)
	}
	defer embedManager.Close()

	// Index all documents
	ctx := context.Background()
	indexed := 0
	skipped := 0

	for _, doc := range app.Documents {
		if err := embedManager.IndexDocument(ctx, doc, *force); err != nil {
			log.Printf("Warning: failed to index document %s: %v", doc.RelPath, err)
		} else {
			indexed++
		}
	}

	if *force {
		log.Printf("Force indexing complete: %d documents indexed", indexed)
	} else {
		log.Printf("Indexing complete: %d documents processed (%d skipped as up-to-date)", indexed, skipped)
	}
}

// printUsage prints the main usage information
func printUsage() {
	fmt.Printf("DimanDocs %s - Documentation browser with semantic search\n\n", Version)
	fmt.Println("Usage:")
	fmt.Println("  dimandocs [options] [config_file]    Start web server")
	fmt.Println("  dimandocs --mcp [config_file]        Start MCP server for Claude")
	fmt.Println("  dimandocs index [options] [config]   Index documents for search")
	fmt.Println("  dimandocs help                       Show this help")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  index       Index documents for semantic search")
	fmt.Println("              Use --force to re-index all documents")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --version   Show version information")
	fmt.Println("  --mcp       Run as MCP server (stdio transport)")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  dimandocs                           Start with dimandocs.json")
	fmt.Println("  dimandocs myconfig.json             Start with custom config")
	fmt.Println("  dimandocs --mcp dimandocs.json      Run MCP server")
	fmt.Println("  dimandocs index                     Index documents")
	fmt.Println("  dimandocs index --force             Force re-index all")
}
