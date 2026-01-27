package service

import "google.golang.org/genai"

// MCPToolsToOpenTool converts an MCPTools struct to an OpenTool with proper JSON schema
func MCPToolsToOpenTool(mcpTool MCPTool) *OpenTool {
	properties := make(map[string]interface{})
	var required []string

	// Use the Properties field which contains the full schema information
	// instead of the Parameters field which only contains string descriptions
	for paramName, schema := range mcpTool.Properties {
		prop := make(map[string]interface{})

		// Set the type
		if schema.Type != "" {
			prop["type"] = schema.Type
		} else if len(schema.Types) > 0 {
			// If multiple types, use the first one
			prop["type"] = schema.Types[0]
		} else {
			// Default to string if no type specified
			prop["type"] = "string"
		}

		// Set the description
		if schema.Description != "" {
			prop["description"] = schema.Description
		}

		// Set default value if present
		if schema.Default != nil {
			prop["default"] = schema.Default
		}

		// Handle enum values
		if len(schema.Enum) > 0 {
			prop["enum"] = schema.Enum
		}

		// Handle array items
		if schema.Items != nil && schema.Type == "array" {
			items := make(map[string]interface{})
			if schema.Items.Type != "" {
				items["type"] = schema.Items.Type
			}
			prop["items"] = items
		}

		properties[paramName] = prop
		required = append(required, paramName)
	}

	parameters := map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}

	return &OpenTool{
		Type: ToolTypeFunction,
		Function: &OpenFunctionDefinition{
			Name:        mcpTool.Name,
			Description: mcpTool.Description,
			Parameters:  parameters,
		},
	}
}

// IsAvailableMCPTool checks if a tool is available in the MCP tools.
func IsAvailableMCPTool(toolName string, client *MCPClient) bool {
	if client == nil {
		return false
	}
	return client.FindTool(toolName) != nil
}

// getMCPTools retrieves all MCP tools from the MCPClient and converts them to OpenTool format
func getMCPTools(client *MCPClient) []*OpenTool {
	var tools []*OpenTool

	servers := client.GetAllServers()
	for _, server := range servers {
		if server.Tools != nil {
			for _, mcpTool := range *server.Tools {
				openTool := MCPToolsToOpenTool(mcpTool)
				tools = append(tools, openTool)
			}
		}
	}

	return tools
}

// getGeminiMCPTools retrieves all MCP tools from the MCPClient and converts them to Gemini functions
func getGeminiMCPTools(client *MCPClient) *genai.Tool {
	if client == nil {
		return nil
	}
	mcpTools := getMCPTools(client)
	var funcs []*genai.FunctionDeclaration

	for _, mcpTool := range mcpTools {
		geminiTool := mcpTool.ToGeminiFunctions()
		funcs = append(funcs, geminiTool)
	}

	return &genai.Tool{
		FunctionDeclarations: funcs,
	}
}

// appendGeminiTool appends new tools to the existing tools
// Tips: gemini tools are grouped together under a single Tool object.
// Because for gemini tool, the tools are for function calling, google search, code execution, etc.
// it is not allowed to have multiple Tools only for function calling.
// so we need to append the new tool's FunctionDeclarations to the existing tool's FunctionDeclarations
func appendGeminiTool(tool *genai.Tool, newtool *genai.Tool) *genai.Tool {
	if tool == nil {
		return newtool
	}
	if newtool == nil {
		return tool
	}
	tool.FunctionDeclarations = append(tool.FunctionDeclarations, newtool.FunctionDeclarations...)
	return tool
}
