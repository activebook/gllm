package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/api/customsearch/v1"
	"google.golang.org/api/option"
)

// Define a struct for each result in the Tavily API response.
type TavilyResult struct {
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	RawContent *string `json:"raw_content"`
}

// Define a struct for the overall Tavily API response.
type TavilyResponse struct {
	Query        string         `json:"query"`
	Answer       string         `json:"answer"`
	Images       []string       `json:"images"`
	Results      []TavilyResult `json:"results"`
	ResponseTime float32        `json:"response_time"` // e.g., "1.67"
}

type TavilyErrorDetail struct {
	Error string `json:"error"`
}

type TavilyError struct {
	Detail TavilyErrorDetail `json:"detail"`
}

var (
	searchApiKey string
	searchCxKey  string
	searchEngine string
)

const TavilyUrl = "https://api.tavily.com/search"
const GoogleSearchEngine = "google"
const BingSearchEngine = "bing"
const TavilySearchEngine = "tavily"

func SetSearchApiKey(key string) {
	searchApiKey = key
}

func SetSearchCxKey(key string) {
	searchCxKey = key
}

func SetSearchEngine(e string) {
	searchEngine = e
}

func GetSearchEngine() string {
	return searchEngine
}

func TavilySearch(query string) (map[string]any, error) {

	// Format the JSON payload, inserting the query variable
	payload := fmt.Sprintf(`{
  "query": "%s",
  "topic": "general",
  "search_depth": "basic",
  "chunks_per_source": 3,
  "max_results": 5,
  "time_range": null,
  "days": 3,
  "include_answer": "basic",
  "include_raw_content": false,
  "include_images": false,
  "include_image_descriptions": false,
  "include_domains": [],
  "exclude_domains": []
}`, query)

	// Create a new POST request with the payload
	req, err := http.NewRequest("POST", TavilyUrl, strings.NewReader(payload))
	if err != nil {
		log.Fatalf("[Tavily]Error creating request: %v", err)
		return nil, fmt.Errorf("[Tavily]Error creating request: %v", err)
	}

	// Insert the token into the header
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", searchApiKey))
	req.Header.Add("Content-Type", "application/json")

	// Execute the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("[Tavily]Error sending request: %v", err)
		return nil, fmt.Errorf("[Tavily]Error sending request: %v", err)
	}
	defer res.Body.Close()

	// Read the response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("[Tavily]Error reading response: %v", err)
		return nil, fmt.Errorf("[Tavily]Error reading response: %v", err)
	}

	fmt.Println("Response Status:", res.Status)
	if res.StatusCode != 200 {
		var tavilyError TavilyError
		if err := json.Unmarshal([]byte(body), &tavilyError); err != nil {
			log.Fatalf("[Tavily]Error parsing JSON: %v", err)
		}
		return nil, fmt.Errorf("[Tavily]Error: %s", tavilyError.Detail.Error)
	}

	var tavilyResp TavilyResponse
	if err := json.Unmarshal([]byte(body), &tavilyResp); err != nil {
		log.Fatalf("[Tavily]Error parsing JSON: %v", err)
		return nil, fmt.Errorf("[Tavily]Error parsing JSON: %v", err)
	}

	formatted, err := tavilyFormatResponse(&tavilyResp)
	if err != nil {
		log.Fatalf("[Tavily]Error formatting response: %v", err)
		return nil, fmt.Errorf("[Tavily]Error formatting response: %v", err)
	}
	return formatted, nil
}

// formatResponse converts a TavilyResponse into the desired response format.
func tavilyFormatResponse(tavilyResp *TavilyResponse) (map[string]any, error) {
	results := make([]any, 0, len(tavilyResp.Results))
	info := map[string]any{
		"answer": tavilyResp.Answer,
	}
	results = append(results, info)

	for _, r := range tavilyResp.Results {
		// Extract displayLink from the URL (e.g., "www.britannica.com")
		displayLink := ""
		parsedURL, err := url.Parse(r.URL)
		if err == nil {
			displayLink = parsedURL.Hostname()
		}
		// Create a map with the desired keys.
		resultMap := map[string]any{
			"title":       r.Title,
			"link":        r.URL,
			"displayLink": displayLink,
			"snippet":     r.Content,
		}
		results = append(results, resultMap)
	}

	response := map[string]any{
		"results":                  results,
		"search_engine_latency_ms": tavilyResp.ResponseTime,
	}
	return response, nil
}

// Alternative approach with explicit conversions for protocol buffer compatibility
func GoogleSearch(query string) (map[string]any, error) {
	// Create results using only types known to work with proto conversion
	ctx := context.Background() // Required for NewService
	svc, err := customsearch.NewService(ctx, option.WithAPIKey(searchApiKey))
	if err != nil {
		log.Fatalf("[Google]Error creating service: %v", err)
		return nil, fmt.Errorf("[Google]Error creating service: %v", err)
	}

	resp, err := svc.Cse.List().Safe("off").Num(5).Cx(searchCxKey).Q(query).Do()
	if err != nil {
		log.Fatalf("[Google]Error making API call: %v", err)
		return nil, fmt.Errorf("[Google]Error making API call: %v", err)
	}

	results := make([]any, 0, len(resp.Items))
	for _, result := range resp.Items {
		resultMap := map[string]any{
			"title":       result.Title,
			"link":        result.Link,
			"displayLink": result.DisplayLink,
			"snippet":     result.Snippet,
		}
		results = append(results, resultMap)
	}

	response := map[string]any{
		"results":                  results,
		"search_engine_latency_ms": float32(resp.SearchInformation.SearchTime), // Using int32 for proto compatibility
	}
	return response, nil
}

// --- Simulation of Google Search ---
func BingSearch(query string) (map[string]any, error) {
	// This is where your search implementation would go
	// For now, we'll return a dummy result
	results := map[string]any{
		"query": query,
		"results": []map[string]string{
			{
				"title":   "Dummy Title",
				"snippet": "Dummy Snippet",
				"url":     "https://www.dummy.com",
			},
		},
		"search_engine_latency_ms": 10,
	}

	return results, nil
}
