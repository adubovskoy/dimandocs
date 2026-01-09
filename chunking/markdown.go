package chunking

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// DefaultMaxChunkSize is the default maximum chunk size in characters
	DefaultMaxChunkSize = 1500
	// DefaultOverlapSize is the default overlap between chunks in characters
	DefaultOverlapSize = 150
	// MinChunkSize is the minimum chunk size to avoid tiny chunks
	MinChunkSize = 100
)

// Chunk represents a text chunk with metadata
type Chunk struct {
	Index        int
	Text         string
	SectionTitle string
	StartOffset  int
	EndOffset    int
}

// Options configures the chunking behavior
type Options struct {
	MaxChunkSize int
	OverlapSize  int
}

// DefaultOptions returns default chunking options
func DefaultOptions() Options {
	return Options{
		MaxChunkSize: DefaultMaxChunkSize,
		OverlapSize:  DefaultOverlapSize,
	}
}

// headerRegex matches markdown headers
var headerRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// ChunkMarkdown splits markdown content into chunks based on headers and size limits
func ChunkMarkdown(content string, opts Options) []Chunk {
	if opts.MaxChunkSize <= 0 {
		opts.MaxChunkSize = DefaultMaxChunkSize
	}
	if opts.OverlapSize < 0 {
		opts.OverlapSize = DefaultOverlapSize
	}

	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var currentSection strings.Builder
	var currentTitle string
	var sectionStart int
	chunkIndex := 0

	flushSection := func(endOffset int) {
		text := strings.TrimSpace(currentSection.String())
		if len(text) < MinChunkSize {
			return
		}

		// Split large sections into smaller chunks
		sectionChunks := splitLargeSection(text, currentTitle, opts, chunkIndex, sectionStart)
		chunks = append(chunks, sectionChunks...)
		chunkIndex += len(sectionChunks)
	}

	offset := 0
	for _, line := range lines {
		lineLen := len(line) + 1 // +1 for newline

		if matches := headerRegex.FindStringSubmatch(line); matches != nil {
			// Found a header, flush current section
			flushSection(offset)

			// Start new section
			currentSection.Reset()
			currentTitle = matches[2]
			sectionStart = offset
		}

		currentSection.WriteString(line)
		currentSection.WriteString("\n")
		offset += lineLen
	}

	// Flush final section
	flushSection(offset)

	// If no chunks were created (no headers), chunk the entire content
	if len(chunks) == 0 && len(strings.TrimSpace(content)) >= MinChunkSize {
		chunks = splitLargeSection(content, "", opts, 0, 0)
	}

	return chunks
}

// splitLargeSection splits a large section into smaller chunks with overlap
func splitLargeSection(text, title string, opts Options, startIndex, startOffset int) []Chunk {
	text = strings.TrimSpace(text)
	if len(text) <= opts.MaxChunkSize {
		return []Chunk{{
			Index:        startIndex,
			Text:         text,
			SectionTitle: title,
			StartOffset:  startOffset,
			EndOffset:    startOffset + len(text),
		}}
	}

	var chunks []Chunk
	chunkIndex := startIndex
	currentOffset := startOffset

	// Split by paragraphs first
	paragraphs := splitIntoParagraphs(text)

	var currentChunk strings.Builder
	chunkStart := currentOffset

	for _, para := range paragraphs {
		paraLen := utf8.RuneCountInString(para)
		currentLen := utf8.RuneCountInString(currentChunk.String())

		// If adding this paragraph would exceed max size
		if currentLen > 0 && currentLen+paraLen+2 > opts.MaxChunkSize {
			// Save current chunk
			chunkText := strings.TrimSpace(currentChunk.String())
			if len(chunkText) >= MinChunkSize {
				chunks = append(chunks, Chunk{
					Index:        chunkIndex,
					Text:         chunkText,
					SectionTitle: title,
					StartOffset:  chunkStart,
					EndOffset:    currentOffset,
				})
				chunkIndex++
			}

			// Start new chunk with overlap
			currentChunk.Reset()
			chunkStart = currentOffset - getOverlapText(chunkText, opts.OverlapSize)

			// Add overlap from previous chunk
			if opts.OverlapSize > 0 && len(chunkText) > opts.OverlapSize {
				overlapText := getLastNChars(chunkText, opts.OverlapSize)
				currentChunk.WriteString(overlapText)
				currentChunk.WriteString("\n\n")
			}
		}

		currentChunk.WriteString(para)
		currentChunk.WriteString("\n\n")
		currentOffset += len(para) + 2
	}

	// Don't forget the last chunk
	chunkText := strings.TrimSpace(currentChunk.String())
	if len(chunkText) >= MinChunkSize {
		chunks = append(chunks, Chunk{
			Index:        chunkIndex,
			Text:         chunkText,
			SectionTitle: title,
			StartOffset:  chunkStart,
			EndOffset:    currentOffset,
		})
	}

	return chunks
}

// splitIntoParagraphs splits text into paragraphs
func splitIntoParagraphs(text string) []string {
	// Split on double newlines
	parts := regexp.MustCompile(`\n\s*\n`).Split(text, -1)
	var paragraphs []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			paragraphs = append(paragraphs, part)
		}
	}

	return paragraphs
}

// getOverlapText returns the length to subtract for overlap calculation
func getOverlapText(text string, overlapSize int) int {
	if len(text) <= overlapSize {
		return len(text)
	}
	return overlapSize
}

// getLastNChars returns the last n characters of a string, breaking at word boundaries
func getLastNChars(s string, n int) string {
	if len(s) <= n {
		return s
	}

	// Start from the end and find a good break point
	start := len(s) - n
	substr := s[start:]

	// Try to break at a space
	if idx := strings.Index(substr, " "); idx != -1 && idx < len(substr)/2 {
		return substr[idx+1:]
	}

	return substr
}

// EstimateTokens provides a rough estimate of tokens for a text
// OpenAI models typically use ~4 characters per token for English
func EstimateTokens(text string) int {
	return len(text) / 4
}
