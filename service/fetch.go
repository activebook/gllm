package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ledongthuc/pdf"
)

// Configuration options for the text extractor
type ExtractorConfig struct {
	UserAgent          string
	HeaderAccept       string
	Timeout            time.Duration
	MinTextLength      int
	BoilerplateIDs     []string
	BoilerplateClasses []string
}

// Default configuration with modern browser headers (Chrome 131, December 2024)
var defaultConfig = ExtractorConfig{
	UserAgent:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	HeaderAccept:       "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/pdf",
	Timeout:            30 * time.Second,
	MinTextLength:      20, // Filter out very short text segments
	BoilerplateIDs:     []string{"header", "footer", "nav", "sidebar", "menu", "comment", "related", "sharing", "social", "advertisement", "ad", "comments", "masthead", "navigation"},
	BoilerplateClasses: []string{"header", "footer", "nav", "sidebar", "menu", "comment", "related", "sharing", "social", "advertisement", "ad", "cookie", "popup", "modal", "overlay", "banner", "notification", "cookie-consent", "gdpr", "privacy-notice", "subscribe", "newsletter", "promo", "comments", "breadcrumb", "pagination", "author-bio", "related-posts", "share-buttons", "widget"},
}

// ExtractTextFromURL fetches a URL and extracts the main text content
// Automatically detects content type and routes to appropriate handler:
// - text/plain, text/markdown: returns content directly
// - application/pdf: extracts text using PDF reader
// - text/html: parses and extracts text with boilerplate removal
func ExtractTextFromURL(url string, config *ExtractorConfig) ([]string, error) {
	if config == nil {
		config = &defaultConfig
	}

	// Setup HTTP client with timeout
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: config.Timeout,
		Jar:     jar,
	}

	// Create request with headers
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set comprehensive browser headers to avoid bot detection
	// These headers mimic a real Chrome browser request
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Accept", config.HeaderAccept)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// Note: We intentionally do NOT set Accept-Encoding here.
	// req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	// Go's http.Transport automatically adds "Accept-Encoding: gzip" and
	// transparently decompresses the response when we don't set it manually.
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// Sec-Fetch-* headers - critical for modern bot detection bypass
	// These indicate a top-level navigation request initiated by user
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	// Additional security headers
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Detect content type and route to appropriate handler
	contentType := resp.Header.Get("Content-Type")
	contentType = strings.ToLower(strings.Split(contentType, ";")[0]) // Remove charset, etc.

	switch {
	case strings.HasPrefix(contentType, "text/plain"),
		strings.HasPrefix(contentType, "text/markdown"),
		strings.HasPrefix(contentType, "text/x-markdown"),
		strings.HasPrefix(contentType, "application/x-yaml"),
		strings.HasPrefix(contentType, "text/yaml"),
		strings.HasPrefix(contentType, "application/json"),
		strings.HasPrefix(contentType, "text/csv"):
		// Plain text content - return as-is
		return extractPlainText(resp.Body, config.MinTextLength)

	case strings.HasPrefix(contentType, "application/pdf"):
		// PDF content - extract text using PDF reader
		return extractPDFText(resp.Body)

	default:
		// Assume HTML content - use goquery parsing
		return extractHTMLText(resp.Body, config)
	}
}

// extractPlainText handles plain text, markdown, JSON, YAML, CSV content
func extractPlainText(body io.Reader, minLength int) ([]string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read plain text: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	var result []string
	for _, line := range lines {
		cleaned := strings.TrimSpace(line)
		if len(cleaned) >= minLength || cleaned != "" {
			result = append(result, cleaned)
		}
	}

	// If no lines met minLength threshold, return all non-empty lines
	if len(result) == 0 {
		for _, line := range lines {
			cleaned := strings.TrimSpace(line)
			if cleaned != "" {
				result = append(result, cleaned)
			}
		}
	}

	return result, nil
}

// extractPDFText extracts text content from a PDF using ledongthuc/pdf
func extractPDFText(body io.Reader) ([]string, error) {
	// Read entire body into bytes (required by pdf library)
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF data: %w", err)
	}

	// Create a reader from bytes
	reader := bytes.NewReader(data)
	pdfReader, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse PDF: %w", err)
	}

	var result []string
	numPages := pdfReader.NumPage()

	for pageNum := 1; pageNum <= numPages; pageNum++ {
		page := pdfReader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			// Skip pages that fail to extract, continue with others
			continue
		}

		// Split by lines and add non-empty ones
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			cleaned := strings.TrimSpace(line)
			if cleaned != "" {
				result = append(result, cleaned)
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no text content extracted from PDF")
	}

	return result, nil
}

// extractHTMLText extracts text from HTML content using goquery
func extractHTMLText(body io.Reader, config *ExtractorConfig) ([]string, error) {
	// Parse HTML document
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Remove likely boilerplate elements by ID and class
	for _, id := range config.BoilerplateIDs {
		doc.Find("#" + id).Remove()
	}

	for _, class := range config.BoilerplateClasses {
		doc.Find("." + class).Remove()
	}

	// Remove common non-content elements including noise-generating tags
	doc.Find("script, style, noscript, iframe, svg, form, button, input, nav, header, footer, aside, template, object, embed, canvas, video, audio, picture, source, track, link, meta, head, figcaption").Remove()

	// Remove JSON-LD and other structured data scripts
	doc.Find("script[type='application/ld+json'], script[type='application/json']").Remove()

	// Extract the main article content
	var textContent []string

	// Common article content selectors (try to find the main content first)
	mainContentSelectors := []string{
		"article", ".article", "#article",
		".content", "#content", ".post-content",
		".story", ".article-body", ".story-body",
		"[itemprop='articleBody']", "[data-testid='article-body']",
	}

	// Try to find the main content area first
	mainContent := doc.Find("body")
	for _, selector := range mainContentSelectors {
		if selection := doc.Find(selector); selection.Length() > 0 {
			mainContent = selection
			break
		}
	}

	// Process text nodes from the main content
	extractTextContent(mainContent, &textContent, config.MinTextLength)

	// If we couldn't find much content, fall back to whole document scanning
	if len(textContent) < 3 {
		textContent = []string{}
		extractTextContent(doc.Find("body"), &textContent, config.MinTextLength)
	}

	return textContent, nil
}

// isHiddenElement checks if an element is hidden via various HTML/CSS mechanisms
func isHiddenElement(el *goquery.Selection) bool {
	// Check inline style for display:none or visibility:hidden
	if style, exists := el.Attr("style"); exists {
		styleLower := strings.ToLower(style)
		if strings.Contains(styleLower, "display:none") ||
			strings.Contains(styleLower, "display: none") ||
			strings.Contains(styleLower, "visibility:hidden") ||
			strings.Contains(styleLower, "visibility: hidden") {
			return true
		}
	}
	// Check hidden attribute
	if _, exists := el.Attr("hidden"); exists {
		return true
	}
	// Check aria-hidden attribute
	if ariaHidden, exists := el.Attr("aria-hidden"); exists && ariaHidden == "true" {
		return true
	}
	return false
}

// skipNodeNames contains tags that should be skipped during text extraction
var skipNodeNames = map[string]bool{
	"script": true, "style": true, "noscript": true, "iframe": true,
	"svg": true, "form": true, "nav": true, "header": true, "footer": true,
	"aside": true, "template": true, "object": true, "embed": true,
	"canvas": true, "video": true, "audio": true, "picture": true,
}

// extractTextContent recursively extracts meaningful text content
func extractTextContent(s *goquery.Selection, results *[]string, minLength int) {
	s.Each(func(_ int, el *goquery.Selection) {
		// Skip hidden elements using comprehensive detection
		if isHiddenElement(el) {
			return
		}

		// Process text directly contained by this element (not in children)
		ownText := getOwnText(el)
		cleanText := cleanupText(ownText)

		if len(cleanText) >= minLength {
			*results = append(*results, cleanText)
		}

		// Process all child elements
		el.Children().Each(func(_ int, child *goquery.Selection) {
			nodeName := goquery.NodeName(child)
			// Skip elements that are likely to contain non-article content
			if !skipNodeNames[nodeName] {
				extractTextContent(child, results, minLength)
			}
		})
	})
}

// getOwnText extracts text directly owned by this element (not from children)
func getOwnText(s *goquery.Selection) string {
	// Clone the selection
	clone := s.Clone()

	// Remove all child nodes
	clone.Children().Remove()

	// Get the remaining text
	return clone.Text()
}

// cleanupText normalizes and cleans text content
func cleanupText(text string) string {
	// Trim whitespace
	text = strings.TrimSpace(text)

	// Replace multiple spaces, newlines, and tabs with a single space
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	return text
}

func fetchWorker(url string) string {
	content, err := ExtractTextFromURL(url, nil)
	if err != nil {
		//Warnf("Can't extracting content from [%s]: %v", url, err)
		Debugf("Error fetching URL [%s]: %v", url, err)
		// Let user know something went wrong
		// Especially if the URL is invalid, 401, 403 errors, etc.
		return fmt.Sprintf("Error fetching URL [%s]: %v", url, err)
	}
	return strings.Join(content, "\n")
}

func FetchProcess(urls []string) []string {
	var wg sync.WaitGroup
	results := make([]string, len(urls))
	resultCh := make(chan struct {
		Index int
		Text  string
	}, len(urls))

	for i, url := range urls {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()
			text := fetchWorker(u)
			resultCh <- struct {
				Index int
				Text  string
			}{Index: idx, Text: text}
		}(i, url)
	}

	wg.Wait()
	close(resultCh)

	for res := range resultCh {
		results[res.Index] = res.Text
	}
	return results
}
