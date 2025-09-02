package service

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// headerTransport is a custom RoundTripper that adds headers to requests
type headerTransport struct {
	headers map[string]string
	base    http.RoundTripper
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}
	if t.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return t.base.RoundTrip(req)
}

type MCPTools struct {
	Name        string
	Description string
	Parameters  map[string]string
}

type MCPSession struct {
	name string
	cs   *mcp.ClientSession
}

type MCPClient struct {
	ctx      context.Context
	client   *mcp.Client
	sessions []*MCPSession
}

// three types of transports supported:
// httpUrl → StreamableHTTPClientTransport
// url → SSEClientTransport
// command → StdioClientTransport
func (mc *MCPClient) Init() {
	mc.ctx = context.Background()

	// Create a new client, with no features.
	mc.client = mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)

}

func (mc *MCPClient) Close() {
	for _, session := range mc.sessions {
		session.cs.Close()
	}
	mc.sessions = []*MCPSession{}
}

func (mc *MCPClient) AddSseServer(name string, url string, headers map[string]string) error {
	// Create HTTP client with custom headers
	httpClient := &http.Client{
		Transport: &headerTransport{
			headers: headers,
		},
	}

	// Create SSE transport
	transport := &mcp.SSEClientTransport{
		Endpoint:   url,
		HTTPClient: httpClient,
	}

	// Connect to the server
	session, err := mc.client.Connect(mc.ctx, transport, nil)
	if err != nil {
		return err
	}
	// Keep track of the session
	mc.sessions = append(mc.sessions, &MCPSession{name, session})
	return nil
}

func (mc *MCPClient) AddHttpServer(name string, url string, headers map[string]string) error {
	// Create HTTP client with custom headers
	httpClient := &http.Client{
		Transport: &headerTransport{
			headers: headers,
		},
	}

	// Create HTTP transport
	transport := &mcp.StreamableClientTransport{
		Endpoint:   url,
		HTTPClient: httpClient,
	}

	// Connect to the server
	session, err := mc.client.Connect(mc.ctx, transport, nil)
	if err != nil {
		return err
	}
	// Keep track of the session
	mc.sessions = append(mc.sessions, &MCPSession{name, session})
	return nil
}

func (mc *MCPClient) AddStdServer(name string, cmd string, args ...string) error {
	// Connect to a server over stdin/stdout
	transport := &mcp.CommandTransport{Command: exec.Command(cmd, args...)}
	session, err := mc.client.Connect(mc.ctx, transport, nil)
	if err != nil {
		return err
	}
	// Keep track of the session
	mc.sessions = append(mc.sessions, &MCPSession{name, session})
	return nil
}

func (mc *MCPClient) FindTool(toolName string) *MCPSession {
	for _, session := range mc.sessions {
		tools, err := session.cs.ListTools(mc.ctx, nil)
		if err != nil {
			continue
		}
		// Find the tool by name, only the first one is returned
		for _, tool := range tools.Tools {
			if tool.Name == toolName {
				return session
			}
		}
	}
	return nil
}

func (mc *MCPClient) CallTool(toolName string, args map[string]any) (string, error) {
	params := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}

	// Find the session by tool name
	session := mc.FindTool(toolName)
	if session == nil {
		return "", fmt.Errorf("no session found for tool %s", toolName)
	}
	//log.Printf("Calling tool %s on session %s", toolName, session.ID())
	res, err := session.cs.CallTool(mc.ctx, params)
	if err != nil {
		return ",", fmt.Errorf("call tool failed: %v", err)
	}

	// Collect the tool's output
	sb := strings.Builder{}
	for _, c := range res.Content {
		sb.WriteString(c.(*mcp.TextContent).Text)
	}
	output := sb.String()

	if res.IsError {
		return "", fmt.Errorf("call tool failed: %v", output)
	}
	return output, nil
}

// Returns a map grouping tools by MCP server session name,
// with each session containing a slice of its available tools.
func (mc *MCPClient) GetAllTools() *map[string]*[]MCPTools {
	allTools := make(map[string]*[]MCPTools)

	// List all tools available on the server
	for _, session := range mc.sessions {
		var mcpTools []MCPTools
		tools, err := session.cs.ListTools(mc.ctx, nil)
		if err != nil {
			continue
		}
		for _, tool := range tools.Tools {
			params := make(map[string]string)
			for k, v := range tool.InputSchema.Properties {
				params[k] = v.String()
			}
			mcpTools = append(mcpTools, MCPTools{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			})
		}
		allTools[session.name] = &mcpTools
	}
	return &allTools
}
