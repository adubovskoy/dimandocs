package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
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