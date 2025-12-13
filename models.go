package main

import (
	"html/template"
	"regexp"
)

// DirectoryConfig represents a directory configuration with path, name, and file pattern
type DirectoryConfig struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	FilePattern string `json:"file_pattern"`
}

// Config represents the application configuration
type Config struct {
	Directories    []DirectoryConfig `json:"directories"`
	Port           string            `json:"port"`
	Title          string            `json:"title"`
	IgnorePatterns []string          `json:"ignore_patterns"`
}

// Document represents a parsed markdown document
type Document struct {
	Title       string
	Path        string
	Content     string
	RelPath     string
	DirName     string
	SourceDir   string
	SourceName  string
	AbsPath     string
	Overview    string
}

// DirectoryGroup represents a group of documents from the same directory
type DirectoryGroup struct {
	Name      string
	Documents []Document
}

// App represents the main application
type App struct {
	Config         Config
	Documents      []Document
	IgnoreRegexes  []*regexp.Regexp
	FileRegexes    map[string]*regexp.Regexp
	WorkingDir     string
}

// IndexData represents data for the index template
type IndexData struct {
	Title          string
	Groups         []DirectoryGroup
	TotalDocuments int
}

// DocumentData represents data for the document template
type DocumentData struct {
	Title    string
	AppTitle string
	DirName  string
	AbsPath  string
	Content  template.HTML
}