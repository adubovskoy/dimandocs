package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

// LoadConfig loads configuration from file and compiles regex patterns
func (a *App) LoadConfig(configFile string) error {
	if configFile == "" {
		configFile = "dimandocs.json"
	}

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, &a.Config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Expand environment variables in config
	a.Config.Embeddings.APIKey = expandEnvVars(a.Config.Embeddings.APIKey)
	a.Config.Embeddings.BaseURL = expandEnvVars(a.Config.Embeddings.BaseURL)
	a.Config.Embeddings.DBPath = expandEnvVars(a.Config.Embeddings.DBPath)

	// Set defaults for embeddings
	if a.Config.Embeddings.DBPath == "" {
		a.Config.Embeddings.DBPath = "embeddings.db"
	}
	if a.Config.Embeddings.Provider == "" {
		a.Config.Embeddings.Provider = "openai"
	}
	if a.Config.Embeddings.Model == "" {
		a.Config.Embeddings.Model = "text-embedding-3-large"
	}

	// Set defaults for MCP
	if a.Config.MCP.Transport == "" {
		a.Config.MCP.Transport = "stdio"
	}

	// Compile ignore patterns
	for _, pattern := range a.Config.IgnorePatterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("failed to compile ignore pattern '%s': %w", pattern, err)
		}
		a.IgnoreRegexes = append(a.IgnoreRegexes, regex)
	}

	// Compile file patterns for each directory
	a.FileRegexes = make(map[string]*regexp.Regexp)
	for _, dirConfig := range a.Config.Directories {
		pattern := dirConfig.FilePattern
		if pattern == "" {
			pattern = "^(?i)(readme\\.md)$" // Default to README.md files
		}
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("failed to compile file pattern '%s' for directory '%s': %w", pattern, dirConfig.Path, err)
		}
		a.FileRegexes[dirConfig.Path] = regex
	}

	return nil
}

// GetWorkingDirectory gets the current working directory
func GetWorkingDirectory() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return workingDir, nil
}

// expandEnvVars expands environment variables in a string
// Supports ${VAR} and $VAR syntax
func expandEnvVars(s string) string {
	if s == "" {
		return s
	}

	// Handle ${VAR} syntax
	result := os.Expand(s, func(key string) string {
		return os.Getenv(key)
	})

	// Also handle $VAR syntax without braces (for simple cases)
	if strings.HasPrefix(result, "$") && !strings.HasPrefix(result, "${") {
		varName := strings.TrimPrefix(result, "$")
		if val := os.Getenv(varName); val != "" {
			return val
		}
	}

	return result
}