package service

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// AtReference represents a single @ reference found in text
type AtReference struct {
	Original string // Original @ reference text (e.g., "@main.go")
	Path     string // Resolved file/directory path
}

// AtRefProcessor handles @ reference processing
type AtRefProcessor struct {
	// Configuration options
	maxFileSize     int64    // Maximum file size to include (bytes)
	maxDirItems     int      // Maximum items to list in directory
	maxDirDepth     int      // Maximum directory depth to recurse
	excludePatterns []string // Patterns to exclude from directory listing
}

// NewAtRefProcessor creates a new @ reference processor
func NewAtRefProcessor() *AtRefProcessor {
	return &AtRefProcessor{
		maxFileSize:     1024 * 1024, // 1MB default, tokens is approximately 0.25million, if read 4 files simutaniously, it will be 1M tokens, so we must limit the file size
		maxDirItems:     100,         // Max 100 items per directory
		maxDirDepth:     3,           // Max 3 levels deep
		excludePatterns: []string{
			// ".git",
			// ".DS_Store",
			// "__pycache__",
			// ".vscode",
			// ".idea",
		},
	}
}

// ParseAtReferences finds all @ references in the given text
func (p *AtRefProcessor) ParseAtReferences(text string) []AtReference {
	// Regex to match @ followed by path (stops at whitespace or common punctuation)
	re := regexp.MustCompile(`@([^\s\)\]}\"',;!?]+)`)
	matches := re.FindAllStringSubmatch(text, -1)

	var references []AtReference
	for _, match := range matches {
		if len(match) >= 2 {
			refPath := match[1]

			references = append(references, AtReference{
				Original: match[0],
				Path:     refPath,
			})
		}
	}

	return references
}

// ProcessReferences processes all @ references and returns augmented text
func (p *AtRefProcessor) ProcessReferences(text string, references []AtReference) (string, error) {
	if len(references) == 0 {
		return text, nil
	}

	var result strings.Builder
	result.WriteString(text)
	result.WriteString("\n\n")

	// Process each reference
	for i, ref := range references {
		if i > 0 {
			result.WriteString("\n")
		}

		content, err := p.resolveReference(ref)
		if err != nil {
			// Include error information but continue processing
			result.WriteString(fmt.Sprintf("=== Error processing %s ===\n", ref.Original))
			result.WriteString(fmt.Sprintf("Error: %v\n\n", err))
			continue
		}

		result.WriteString(content)
	}

	return result.String(), nil
}

// resolveReference resolves a single @ reference to its content
func (p *AtRefProcessor) resolveReference(ref AtReference) (string, error) {
	// Resolve the path (handle relative paths)
	fullPath, err := p.resolvePath(ref.Path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %s: %v", ref.Path, err)
	}

	// Check if path exists
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("path does not exist: %s", fullPath)
	}

	// Determine type based on actual filesystem info
	if info.IsDir() {
		return p.processDirectory(fullPath, 0)
	} else {
		return p.processFile(fullPath, info)
	}
}

// resolvePath resolves relative paths to absolute paths
func (p *AtRefProcessor) resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	// For relative paths, resolve relative to current working directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

// processFile processes a file reference
func (p *AtRefProcessor) processFile(fullPath string, info os.FileInfo) (string, error) {
	// Check file size
	if info.Size() > p.maxFileSize {
		return "", fmt.Errorf("file too large (%d bytes, max %d bytes): %s",
			info.Size(), p.maxFileSize, fullPath)
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	// Get MIME type
	mimeType := GetMIMEType(fullPath)

	// Format the output
	var result strings.Builder
	result.WriteString(fmt.Sprintf("=== File: %s ===\n", fullPath))

	// Add metadata
	result.WriteString(fmt.Sprintf("Size: %d bytes, Type: %s\n", info.Size(), mimeType))

	// Add content
	result.WriteString("```\n")
	result.WriteString(string(content))
	result.WriteString("\n```\n")

	return result.String(), nil
}

// processDirectory processes a directory reference
func (p *AtRefProcessor) processDirectory(fullPath string, depth int) (string, error) {
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %v", err)
	}

	// Filter and sort entries
	var filteredEntries []os.DirEntry
	for _, entry := range entries {
		if !p.shouldExclude(entry.Name()) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	// Limit number of entries
	if len(filteredEntries) > p.maxDirItems {
		filteredEntries = filteredEntries[:p.maxDirItems]
	}

	// Sort entries (directories first, then files, alphabetically)
	sort.Slice(filteredEntries, func(i, j int) bool {
		nameI, nameJ := filteredEntries[i].Name(), filteredEntries[j].Name()

		// Directories first
		iIsDir := filteredEntries[i].IsDir()
		jIsDir := filteredEntries[j].IsDir()
		if iIsDir != jIsDir {
			return iIsDir
		}

		// Then alphabetically
		return nameI < nameJ
	})

	// Format the output
	var result strings.Builder
	if depth == 0 {
		result.WriteString(fmt.Sprintf("=== Directory: %s ===\n", fullPath))
	}

	// Add all items under the directory
	allPaths := p.processSubDir(fullPath)

	// Format as a simple list of absolute paths
	for _, path := range allPaths {
		result.WriteString(fmt.Sprintf("%s\n", path))
	}

	return result.String(), nil
}

// processSubDir returns a list of all absolute paths (files and directories) under the given directory
func (p *AtRefProcessor) processSubDir(fullPath string) []string {
	var paths []string

	filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip on error
		}

		// Skip the root directory itself
		if path == fullPath {
			return nil
		}

		// Check depth limit
		relPath, err := filepath.Rel(fullPath, path)
		if err != nil {
			return nil
		}
		currentDepth := strings.Count(relPath, string(filepath.Separator))
		if currentDepth > p.maxDirDepth {
			return filepath.SkipDir
		}

		// Apply exclusion patterns
		if p.shouldExclude(info.Name()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Limit total items
		if len(paths) >= p.maxDirItems {
			return filepath.SkipDir
		}

		paths = append(paths, path)
		return nil
	})

	return paths
}

// shouldExclude checks if a file/directory should be excluded from listing
func (p *AtRefProcessor) shouldExclude(name string) bool {
	for _, pattern := range p.excludePatterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}

	// Also exclude hidden files/directories (starting with .)
	// if strings.HasPrefix(name, ".") {
	// 	return true
	// }

	return false
}

// ProcessText processes text containing @ references and returns augmented text
func (p *AtRefProcessor) ProcessText(text string) (string, error) {
	references := p.ParseAtReferences(text)
	return p.ProcessReferences(text, references)
}

// SetMaxFileSize sets the maximum file size to include
func (p *AtRefProcessor) SetMaxFileSize(size int64) {
	p.maxFileSize = size
}

// SetMaxDirItems sets the maximum number of directory items to list
func (p *AtRefProcessor) SetMaxDirItems(count int) {
	p.maxDirItems = count
}

// AddExcludePattern adds a pattern to exclude from directory listings
func (p *AtRefProcessor) AddExcludePattern(pattern string) {
	p.excludePatterns = append(p.excludePatterns, pattern)
}
