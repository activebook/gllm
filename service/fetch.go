package service

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
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

// Default configuration
var defaultConfig = ExtractorConfig{
	UserAgent:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	HeaderAccept:       "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
	Timeout:            30 * time.Second,
	MinTextLength:      20, // Filter out very short text segments
	BoilerplateIDs:     []string{"header", "footer", "nav", "sidebar", "menu", "comment", "related", "sharing", "social", "advertisement", "ad"},
	BoilerplateClasses: []string{"header", "footer", "nav", "sidebar", "menu", "comment", "related", "sharing", "social", "advertisement", "ad", "cookie", "popup"},
}

// ExtractTextFromURL fetches a URL and extracts the main text content
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

	// Set common headers to mimic a real browser
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Accept", config.HeaderAccept)
	//req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
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

	// Remove common non-content elements
	doc.Find("script, style, noscript, iframe, svg, form, button, input").Remove()

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

// extractTextContent recursively extracts meaningful text content
func extractTextContent(s *goquery.Selection, results *[]string, minLength int) {
	s.Each(func(_ int, el *goquery.Selection) {
		// Skip hidden elements
		if display, exists := el.Attr("style"); exists &&
			(strings.Contains(display, "display:none") || strings.Contains(display, "display: none")) {
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
			if nodeName != "script" && nodeName != "style" && nodeName != "noscript" &&
				nodeName != "iframe" && nodeName != "svg" && nodeName != "form" {
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
		Debugf("Can't extracting content from [%s]: %v", url, err)
		return ""
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
