package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	serp "github.com/serpapi/google-search-results-golang"
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
	searchApiKey  string
	searchCxKey   string
	searchEngine  string
	maxReferences int
)

const TavilyUrl = "https://api.tavily.com/search"
const GoogleSearchEngine = "google"
const BingSearchEngine = "bing"
const TavilySearchEngine = "tavily"
const NoneSearchEngine = "none"

func GetDefaultSearchEngineName() string {
	return GoogleSearchEngine
}

func GetNoneSearchEngineName() string {
	// Using for placeholder
	return NoneSearchEngine
}

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

func SetMaxReferences(max int) {
	maxReferences = max
}

func TavilySearch(query string) (map[string]any, error) {

	// Format the JSON payload, inserting the query variable
	payload := fmt.Sprintf(`{
  "query": "%s",
  "topic": "general",
  "search_depth": "basic",
  "chunks_per_source": 3,
  "max_results": 10,
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
		Errorf("[Tavily]Error creating request: %v", err)
		return nil, fmt.Errorf("[Tavily]Error creating request: %v", err)
	}

	// Insert the token into the header
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", searchApiKey))
	req.Header.Add("Content-Type", "application/json")

	// Execute the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		Errorf("[Tavily]Error sending request: %v", err)
		return nil, fmt.Errorf("[Tavily]Error sending request: %v", err)
	}
	defer res.Body.Close()

	// Read the response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		Errorf("[Tavily]Error reading response: %v", err)
		return nil, fmt.Errorf("[Tavily]Error reading response: %v", err)
	}

	if res.StatusCode != 200 {
		var tavilyError TavilyError
		if err := json.Unmarshal([]byte(body), &tavilyError); err != nil {
			Errorf("[Tavily]Error parsing JSON: %v", err)
		}
		return nil, fmt.Errorf("[Tavily]Error: %s", tavilyError.Detail.Error)
	}

	var tavilyResp TavilyResponse
	if err := json.Unmarshal([]byte(body), &tavilyResp); err != nil {
		Errorf("[Tavily]Error parsing JSON: %v", err)
		return nil, fmt.Errorf("[Tavily]Error parsing JSON: %v", err)
	}

	formatted, err := tavilyFormatResponse(&tavilyResp)
	if err != nil {
		Errorf("[Tavily]Error formatting response: %v", err)
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

	// Collect all links
	links := make([]string, 0, len(tavilyResp.Results))
	for _, result := range tavilyResp.Results {
		links = append(links, result.URL)
	}

	// Fetch contents for all links
	contents := FetchProcess(links)

	// Convert results to map[string]any
	for i, r := range tavilyResp.Results {
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
			"content":     contents[i], // Attach fetched content here
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
		Errorf("[Google]Error creating service: %v", err)
		return nil, fmt.Errorf("[Google]Error creating service: %v", err)
	}

	resp, err := svc.Cse.List().Safe("off").Num(10).Cx(searchCxKey).Q(query).Do()
	if err != nil {
		Errorf("[Google]Error making API call: %v", err)
		return nil, fmt.Errorf("[Google]Error making API call: %v", err)
	}

	// Collect all links
	links := make([]string, 0, len(resp.Items))
	for _, result := range resp.Items {
		links = append(links, result.Link)
	}

	// Fetch contents for all links
	contents := FetchProcess(links)

	// Convert results to map[string]any
	results := make([]any, 0, len(resp.Items))
	for i, result := range resp.Items {
		resultMap := map[string]any{
			"title":       result.Title,
			"link":        result.Link,
			"displayLink": result.DisplayLink,
			"snippet":     result.Snippet,
			"content":     contents[i], // Attach fetched content here
		}
		results = append(results, resultMap)
	}

	response := map[string]any{
		"results":                  results,
		"search_engine_latency_ms": float32(resp.SearchInformation.SearchTime), // Using int32 for proto compatibility
	}
	return response, nil
}

// --- Simulation of Bing Search ---
func BingSearch(query string) (map[string]any, error) {
	// Call SerpAPI Search API
	return SerpAPISearch(query, "bing")
}

func SerpAPISearch(query string, engine string) (map[string]any, error) {
	parameter := map[string]string{
		"engine":     engine,
		"q":          query,
		"count":      "10",
		"first":      "1",
		"safeSearch": "off",
	}

	search := serp.NewGoogleSearch(parameter, searchApiKey)
	results, err := search.GetJSON()
	if err != nil {
		//Errorf("[SerpAPI]Error getting JSON: %v", err)
		return nil, fmt.Errorf("[SerpAPI]Error getting JSON: %v", err)
	}
	organic_results := results["organic_results"]
	search_meta := results["search_metadata"]
	var total_time_taken float32
	if metaMap, ok := search_meta.(map[string]interface{}); ok {
		if totalTime, ok := metaMap["total_time_taken"].(float64); ok {
			total_time_taken = float32(totalTime)
		}
	}

	formatted_results, err := formatSerpAPIResponse(organic_results)
	if err != nil {
		Errorf("[SerpAPI]Error formatting response: %v", err)
		return nil, fmt.Errorf("[SerpAPI]Error formatting response: %v", err)
	}
	response := map[string]any{
		"results":                  formatted_results,
		"search_engine_latency_ms": total_time_taken,
	}

	return response, nil
}

// formatSerpAPIResponse formats the SerpAPI response into a standardized structure
func formatSerpAPIResponse(organic_results interface{}) (map[string]any, error) {
	result := make(map[string]any)

	// Extract organic search results
	results := make([]any, 0)
	if organicResults, ok := organic_results.([]interface{}); ok {
		for _, r := range organicResults {
			if result, ok := r.(map[string]interface{}); ok {
				formattedResult := map[string]interface{}{
					"title":       GetStringValue(result, "title"),
					"link":        GetStringValue(result, "link"),
					"displayLink": GetStringValue(result, "displayed_link"),
					"snippet":     GetStringValue(result, "snippet"),
				}
				// Add source domain
				if link, ok := result["link"].(string); ok {
					if parsedURL, err := url.Parse(link); err == nil {
						formattedResult["source"] = parsedURL.Hostname()
					}
				}
				results = append(results, formattedResult)
			}
		}
	}

	result["results"] = results
	return result, nil
}

func NoneSearch(query string) (map[string]any, error) {
	return map[string]any{
		"query":                    query,
		"results":                  []map[string]string{},
		"search_engine_latency_ms": 0,
	}, nil
}

func RetrieveReferences(references []*map[string]any) string {
	if len(references) == 0 {
		return ""
	}
	sb := strings.Builder{}
	sb.WriteString("### References:\n")
	index, total := 0, 0
	for _, ref := range references {
		if ref == nil {
			continue
		}
		if results, ok := (*ref)["results"].([]any); ok {
			for _, result := range results {
				if linkMap, ok := result.(map[string]any); ok {
					link, hasLink := linkMap["link"].(string)
					title, hasTitle := linkMap["title"].(string)
					displayLink, hasDisplayLink := linkMap["displayLink"].(string)
					source := displayLink
					if !hasDisplayLink {
						source = link
					}
					if hasLink && link != "" {
						total++
						if index < maxReferences {
							index++
							// Markdown: 1. **[Title](URL)**
							//           Source: [Source](URL)
							desc := source
							if hasTitle && title != "" {
								desc = title
							}
							sb.WriteString(fmt.Sprintf("%d. **%s**  \n   Source: [%s](%s)\n",
								index,
								TruncateString(desc, 80),
								TruncateString(source, 30),
								link,
							))
						}
					}
				}
			}
		}
	}
	if total > maxReferences {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("> **...and %d more references. Use the `-r` flag to view more.**\n", total-maxReferences))
	}
	return sb.String()
}
