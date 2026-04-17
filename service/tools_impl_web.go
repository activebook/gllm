package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func webFetchToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	if err := CheckToolPermission(ToolWebFetch, argsMap); err != nil {
		return "", err
	}

	url, ok := (*argsMap)["url"].(string)
	if !ok {
		return "", fmt.Errorf("url not found in arguments")
	}

	// Call the fetch function
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	results := FetchProcess(ctx, []string{url})

	// Check if FetchProcess returned any results
	if len(results) == 0 {
		return fmt.Sprintf("Failed to fetch content from %s: no results returned.", url), nil
	}

	res := results[0]
	if res.Error != nil {
		return fmt.Sprintf("Error fetching content from %s: %v", url, res.Error), nil
	}

	if res.Content == "" {
		return "Fetched content is empty.", nil
	}

	// Create and return the tool response message
	return fmt.Sprintf("Fetched content from %s:\n%s", url, res.Content), nil
}

func webSearchToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolWebSearch, argsMap); err != nil {
		return "", err
	}

	query, ok := (*argsMap)["query"].(string)
	if !ok {
		return "", fmt.Errorf("query not found in arguments")
	}

	// Call the search function
	engine := op.search.Name
	var data map[string]any
	var err error
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
		data, err = op.search.GoogleSearch(query)
	case BingSearchEngine:
		// Use Bing Search Engine
		data, err = op.search.BingSearch(query)
	case TavilySearchEngine:
		// Use Tavily Search Engine
		data, err = op.search.TavilySearch(query)
	case NoneSearchEngine:
		// Use None Search Engine
		data, err = op.search.NoneSearch(query)
	default:
		err = fmt.Errorf("unknown search engine: %s", engine)
	}

	if err != nil {
		return "", fmt.Errorf("error performing search for query '%s': %v", query, err)
	}
	// keep the search results for references
	op.queries = append(op.queries, query)
	op.references = append(op.references, data)

	// Convert search results to JSON string
	resultsJSON, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshaling search results for query '%s': %v", query, err)
	}

	return string(resultsJSON), nil
}
