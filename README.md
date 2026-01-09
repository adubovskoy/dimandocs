# DimanDocs

A lightweight web-based documentation browser for markdown files. DimanDocs scans multiple directories for markdown files and presents them in a clean, organized web interface with automatic overview extraction.

## Features

- **Single binary**: All templates embedded - no external files needed
- **Simple deployment**: Just copy the binary and config file anywhere
- **Multi-directory scanning**: Monitor multiple directories simultaneously
- **Flexible file patterns**: Use regex patterns to match specific markdown files
- **Overview extraction**: Automatically extracts and displays the first paragraph after "## Overview" heading
- **Grouped display**: Documents are organized by their source directories
- **Path normalization**: Displays clean, absolute paths for easy navigation
- **Ignore patterns**: Exclude unwanted directories (node_modules, .git, etc.)
- **Responsive UI**: Clean grid layout with hover effects
- **Markdown rendering**: Full markdown support using Blackfriday
- **MCP Server**: Chat with your documentation using Claude via Model Context Protocol
- **Semantic Search**: Vector-based search using OpenAI, Voyage AI, or Ollama embeddings

## Quick Start

### Option 1: Download Pre-built Binary (Recommended)

Download the latest release from GitHub:

1. Go to [Releases](https://github.com/adubovskoy/dimandocs/releases)
2. Download the binary for your platform
3. Extract and run `./dimandocs`

**Available Platforms**:
- Linux AMD64 (`dimandocs-linux-amd64`)
- macOS Apple Silicon (`dimandocs-macos-arm64`)
- Windows AMD64 (`dimandocs-windows-amd64.exe`)

### Option 2: Build from Source

```bash
go build
```

This creates the `dimandocs` executable.

### Run

```bash
./dimandocs [config_file]
```

If no config file is specified, it defaults to `dimandocs.json` in the current directory.

**Note**: The binary is self-contained with embedded templates. You only need the `dimandocs` binary and `dimandocs.json` config file - no need to copy the `templates/` directory!

### Version Information

Check the version:

```bash
./dimandocs -version
```

### Open in Browser

Once running, open your browser to:

```bash
xdg-open http://localhost:8090
```

Or manually navigate to `http://localhost:8090` (or the port specified in your config).

## Configuration

Create a `dimandocs.json` file with the following structure:

```json
{
  "directories": [
    {
      "path": "./kvlu/ADRs",
      "name": "ADRs",
      "file_pattern": "\\.md$"
    },
    {
      "path": "./drupal/web/modules/custom",
      "name": "Custom Drupal Modules",
      "file_pattern": "^(?i)(readme\\.md)$"
    }
  ],
  "port": "8090",
  "title": "Documentation Browser",
  "ignore_patterns": [
    ".*/node_modules/.*",
    ".*/\\.git/.*",
    ".*/vendor/.*"
  ]
}
```

### Configuration Options

#### directories (array, required)
List of directories to scan for documentation files.

- **path** (string): Relative or absolute path to the directory
- **name** (string): Display name for this documentation group
- **file_pattern** (string): Regex pattern to match files
  - Default: `^(?i)(readme\\.md)$` (matches README.md files)
  - Example: `\\.md$` (matches all .md files)
  - Example: `^(?i)(readme|contributing)\\.md$` (matches README.md or CONTRIBUTING.md)

#### port (string, optional)
Port number for the web server. Default: `"8080"`

#### title (string, optional)
Title displayed in the web interface. Default: `"Documentation Browser"`

#### ignore_patterns (array, optional)
Regex patterns for paths to ignore during scanning. Common patterns:
- `.*/node_modules/.*` - Node.js dependencies
- `.*/\\.git/.*` - Git repository files
- `.*/vendor/.*` - Vendor dependencies
- `.*/build/.*` - Build outputs
- `.*/dist/.*` - Distribution files

#### embeddings (object, optional)
Configuration for semantic search and MCP server:

```json
{
  "embeddings": {
    "enabled": true,
    "provider": "openai",
    "model": "text-embedding-3-large",
    "api_key": "${OPENAI_API_KEY}",
    "db_path": "embeddings.db"
  }
}
```

**Supported providers:**

| Provider | `provider` value | API Key Env Var | Models |
|----------|-----------------|-----------------|--------|
| OpenAI | `openai` | `OPENAI_API_KEY` | `text-embedding-3-large` (3072d), `text-embedding-3-small` (1536d) |
| Voyage AI | `voyage` | `VOYAGE_API_KEY` | `voyage-3` (1024d), `voyage-code-3` (1024d), `voyage-3-lite` (512d) |
| Ollama | `ollama` | not required | `nomic-embed-text` (768d), `mxbai-embed-large` (1024d) |

**API Key auto-detection:** If `api_key` is not specified in config, DimanDocs automatically reads from the standard environment variable based on provider (`OPENAI_API_KEY`, `VOYAGE_API_KEY`). This means you can omit `api_key` from `dimandocs.json` entirely.

#### mcp (object, optional)
MCP server configuration:

```json
{
  "mcp": {
    "enabled": true,
    "transport": "stdio"
  }
}
```

## MCP Integration (Chat with Documentation)

DimanDocs includes an MCP (Model Context Protocol) server that allows Claude to search and read your documentation.

### Configuration Locations

| Application | Config File Location |
|-------------|---------------------|
| Claude CLI (global) | `~/.claude/settings.json` |
| Claude CLI (project) | `<project>/.claude/settings.json` or `<project>/.mcp.json` |
| Claude Desktop (Linux) | `~/.config/claude-desktop/config.json` |
| Claude Desktop (macOS) | `~/Library/Application Support/Claude/claude_desktop_config.json` |

### Provider Setup Examples

#### OpenAI (Recommended for quality)

**dimandocs.json** (no api_key needed - auto-detected from env):
```json
{
  "directories": [...],
  "embeddings": {
    "enabled": true,
    "provider": "openai",
    "model": "text-embedding-3-large"
  },
  "mcp": { "enabled": true }
}
```

**Claude CLI/Desktop config:**
```json
{
  "mcpServers": {
    "dimandocs": {
      "command": "/path/to/dimandocs",
      "args": ["--mcp", "/path/to/dimandocs.json"],
      "env": { "OPENAI_API_KEY": "sk-..." }
    }
  }
}
```

**Available models:**
- `text-embedding-3-large` - 3072 dimensions, best quality
- `text-embedding-3-small` - 1536 dimensions, faster & cheaper

---

#### Voyage AI (Recommended for code)

**dimandocs.json** (no api_key needed - auto-detected from env):
```json
{
  "directories": [...],
  "embeddings": {
    "enabled": true,
    "provider": "voyage",
    "model": "voyage-3"
  },
  "mcp": { "enabled": true }
}
```

**Claude CLI/Desktop config:**
```json
{
  "mcpServers": {
    "dimandocs": {
      "command": "/path/to/dimandocs",
      "args": ["--mcp", "/path/to/dimandocs.json"],
      "env": { "VOYAGE_API_KEY": "pa-..." }
    }
  }
}
```

**Available models:**
- `voyage-3` - 1024 dimensions, general purpose
- `voyage-code-3` - 1024 dimensions, optimized for code
- `voyage-3-lite` - 512 dimensions, faster & cheaper

---

#### Ollama (Free, Local, No API key)

1. **Install Ollama:**
   ```bash
   curl -fsSL https://ollama.com/install.sh | sh
   ollama pull nomic-embed-text
   ```

2. **dimandocs.json:**
   ```json
   {
     "directories": [...],
     "embeddings": {
       "enabled": true,
       "provider": "ollama",
       "model": "nomic-embed-text",
       "db_path": "embeddings.db"
     },
     "mcp": {
       "enabled": true,
       "transport": "stdio"
     }
   }
   ```

3. **Claude CLI/Desktop config (no env needed):**
   ```json
   {
     "mcpServers": {
       "dimandocs": {
         "command": "/path/to/dimandocs",
         "args": ["--mcp", "/path/to/dimandocs.json"]
       }
     }
   }
   ```

**Available models:**
- `nomic-embed-text` - 768 dimensions, general purpose
- `mxbai-embed-large` - 1024 dimensions, higher quality

---

### Project-Specific MCP Configuration

To add MCP only for a specific project, create `.claude/settings.json` or `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "my-project-docs": {
      "command": "/path/to/dimandocs",
      "args": ["--mcp", "/path/to/project/dimandocs.json"],
      "env": {
        "VOYAGE_API_KEY": "pa-..."
      }
    }
  }
}
```

Then run `claude` from your project directory - the MCP server will be available only in that project.

### MCP Tools Available

Once configured, Claude can use these tools:

| Tool | Description |
|------|-------------|
| `search_docs` | Semantic search across all documentation |
| `get_document` | Get full content of a specific document |
| `list_documents` | List all available documents |

### Running MCP Server Standalone

```bash
# Run as MCP server only (for Claude CLI/Desktop)
./dimandocs --mcp dimandocs.json

# Run web server with embeddings (normal mode)
./dimandocs dimandocs.json
```

### Indexing Documents

Use the `index` command to pre-index documents before using MCP:

```bash
# Index documents (skips unchanged)
./dimandocs index dimandocs.json

# Force re-index all documents
./dimandocs index --force dimandocs.json

# Show help
./dimandocs index --help
```

**When to use `index`:**
- Before first MCP use to pre-build the search index
- After adding many new documents
- After changing embedding provider (dimension change triggers automatic re-index)
- With `--force` to rebuild index from scratch

## How It Works

### Application Logic

1. **Initialization**
   - Loads and parses `dimandocs.json` (or specified config file)
   - Compiles all regex patterns (file patterns and ignore patterns)
   - Stores working directory for path calculations
   - Uses embedded templates (no external template files needed)

2. **Directory Scanning**
   - Recursively walks each configured directory
   - Applies ignore patterns to skip unwanted paths
   - Matches files against the directory's file pattern
   - Processes each matching markdown file

3. **Document Processing**
   - Reads markdown file content
   - Extracts the first `# Heading` as document title
   - Extracts overview paragraph (first paragraph after `## Overview`)
   - Calculates relative and absolute paths
   - Normalizes paths (replaces `../` prefix with `/`)

4. **Web Interface**
   - Groups documents by their source directory configuration
   - Displays documents in a responsive grid layout
   - Renders markdown content using Blackfriday library

### Path Display Logic

The application displays paths relative to the working directory where DimanDocs is executed. Special handling:

- **Relative paths within working dir**: Displayed as-is (e.g., `kvlu/ADRs`)
- **Paths outside working dir**: `../` prefix is replaced with `/` (e.g., `../drupal/modules` becomes `/drupal/modules`)

### Overview Extraction

The overview feature looks for `## Overview` heading and extracts the first paragraph:

```markdown
# My Module

## Overview

This is the overview paragraph that will be extracted
and displayed on the index page as a preview.

## Installation

...
```

The extracted text: "This is the overview paragraph that will be extracted and displayed on the index page as a preview."

## Project Structure

```
dimandocs/
├── main.go           # Application entry point
├── app.go            # Core application logic, HTTP handlers, embedded templates
├── config.go         # Configuration loading and validation
├── models.go         # Data structures (Config, Document, etc.)
├── dimandocs.json    # Configuration file
├── templates/        # Templates (embedded into binary)
│   ├── index.html    # Document listing page
│   └── document.html # Individual document view
└── README.md         # This file
```

**Deployment**: Only `dimandocs` (binary) and `dimandocs.json` (config) are needed. The `templates/` directory is embedded in the binary during build.

## API Routes

- `GET /` - Index page showing all documents grouped by directory
- `GET /doc/{path}` - View individual document with rendered markdown
- `GET /static/*` - Static file serving (if needed)


### Dependencies

- Go 1.13+ (for `ioutil` compatibility)
- [Blackfriday v2](https://github.com/russross/blackfriday) - Markdown rendering

### Adding Features

The application is structured for easy extension:

- **New document metadata**: Add fields to `Document` struct in `models.go`
- **Custom processing**: Modify `processFile()` in `app.go`
- **UI customization**: Edit templates in `templates/` directory
- **Additional routes**: Add handlers in `SetupRoutes()` method


## License

MIT
