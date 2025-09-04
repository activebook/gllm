package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
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
	Properties  map[string]*jsonschema.Schema // Keep origin JSON Schema
}

type MCPServer struct {
	Name    string
	Allowed bool
	Tools   *[]MCPTools
}

type MCPSession struct {
	name string
	cs   *mcp.ClientSession
}

type MCPClient struct {
	ctx           context.Context
	client        *mcp.Client
	sessions      []*MCPSession
	servers       []*MCPServer
	toolToSession map[string]*MCPSession
}

/*
A singleton pattern for the MCP client is an excellent approach.
Since MCP functionality is independent of the LLM model and conversation context,
a single shared instance can serve all requests across the application.
*/
var (
	mcpClient     *MCPClient
	mcpClientOnce sync.Once
)

func GetMCPClient() *MCPClient {
	mcpClientOnce.Do(func() {
		mcpClient = &MCPClient{}
	})
	return mcpClient
}

// three types of transports supported:
// httpUrl → StreamableHTTPClientTransport
// url → SSEClientTransport
// command → StdioClientTransport
// Only want list all servers, unless loadAll is false, then only load allowed servers
func (mc *MCPClient) Init(loadAll bool) error {
	if mc.client != nil {
		// already initialized
		return nil
	}

	mc.ctx = context.Background()
	mc.toolToSession = make(map[string]*MCPSession)

	// Create a new client, with no features.
	mc.client = mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)

	var err error
	config, err := LoadMCPServers()
	if err != nil {
		return err
	}

	if config == nil {
		//return fmt.Errorf("no MCP configuration found")
		return nil
	}

	// Load mcp servers
	servers := []*MCPServer{}
	// Connect to each server based on its type
	for serverName, server := range config.MCPServers {
		// Skip if not in allowed list (if allow list is not empty)
		if !server.Allowed && !loadAll {
			continue
		}

		if server.Type == "sse" || server.Url != "" || server.BaseUrl != "" {
			// Add SSE server
			err = mc.AddSseServer(serverName, server.BaseUrl, server.Headers)
		} else if server.Type == "std" || server.Type == "local" || server.Command != "" {
			// Add stdio server
			dir := server.WorkDir
			if dir == "" {
				dir = server.Cwd
			}
			err = mc.AddStdServer(serverName, server.Command, server.Env, dir, server.Args...)
		} else if server.Type == "http" || server.HttpUrl != "" {
			// Add HTTP server
			err = mc.AddHttpServer(serverName, server.HttpUrl, server.Headers)
		}

		if err != nil {
			// don't continue with other servers
			return err
		}
		session := mc.sessions[len(mc.sessions)-1]
		tools, err := mc.GetTools(session)
		if err != nil {
			return err
		}
		// Populate tool to session map for fast lookup
		if tools != nil {
			for _, tool := range *tools {
				mc.toolToSession[tool.Name] = session
			}
		}
		// Keep tools null for now, will be populated when listing
		servers = append(servers, &MCPServer{Name: serverName, Allowed: server.Allowed, Tools: tools})
	}
	mc.servers = servers
	return nil
}

func (mc *MCPClient) Close() {
	for _, session := range mc.sessions {
		session.cs.Close()
	}
	mc.sessions = []*MCPSession{}
	mc.toolToSession = nil
	mc.client = nil
	mc.ctx = nil
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

func (mc *MCPClient) AddStdServer(name string, cmd string, env map[string]string, cwd string, args ...string) error {
	// Connect to a server over stdin/stdout
	transport := &mcp.CommandTransport{Command: exec.Command(cmd, args...)}

	// Set the environment variables
	transport.Command.Env = os.Environ()
	for k, v := range env {
		transport.Command.Env = append(transport.Command.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set the working directory
	transport.Command.Dir = cwd

	// Connect to the server
	session, err := mc.client.Connect(mc.ctx, transport, nil)
	if err != nil {
		return err
	}
	// Keep track of the session
	mc.sessions = append(mc.sessions, &MCPSession{name, session})
	return nil
}

func (mc *MCPClient) FindTool(toolName string) *MCPSession {
	return mc.toolToSession[toolName]
}

type MCPToolResponseType string

const (
	MCPResponseText  MCPToolResponseType = "text"
	MCPResponseImage MCPToolResponseType = "image"
	MCPResponseAudio MCPToolResponseType = "audio"
)

type MCPToolResponse struct {
	Types    []MCPToolResponseType
	Contents []string
}

func (mc *MCPClient) CallTool(toolName string, args map[string]any) (*MCPToolResponse, error) {
	params := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}

	// Find the session by tool name
	session := mc.FindTool(toolName)
	if session == nil {
		return nil, fmt.Errorf("no session found for tool %s", toolName)
	}
	//log.Printf("Calling tool %s on session %s", toolName, session.ID())
	res, err := session.cs.CallTool(mc.ctx, params)
	if err != nil {
		return nil, fmt.Errorf("call tool failed: %v", err)
	}

	response := &MCPToolResponse{}

	for _, c := range res.Content {
		if cc, ok := c.(*mcp.TextContent); ok {
			response.Types = append(response.Types, MCPResponseText)
			response.Contents = append(response.Contents, cc.Text)
		} else if cc, ok := c.(*mcp.ImageContent); ok {
			base64Data := base64.StdEncoding.EncodeToString(cc.Data)
			str := fmt.Sprintf("data:%s;base64,%s", cc.MIMEType, base64Data)
			response.Types = append(response.Types, MCPResponseImage)
			response.Contents = append(response.Contents, str)
		} else if cc, ok := c.(*mcp.AudioContent); ok {
			base64Data := base64.StdEncoding.EncodeToString(cc.Data)
			str := fmt.Sprintf("data:%s;base64,%s", cc.MIMEType, base64Data)
			response.Types = append(response.Types, MCPResponseAudio)
			response.Contents = append(response.Contents, str)
		} else {
			response.Types = append(response.Types, MCPResponseText)
			response.Contents = append(response.Contents, "Unknown content type")
		}
	}

	if res.IsError {
		return nil, fmt.Errorf("call tool failed: %v", response.Contents)
	}
	return response, nil
}

// Returns a map grouping tools by MCP server session name,
// with each session containing a slice of its available tools.
func (mc *MCPClient) GetAllServers() []*MCPServer {
	return mc.servers
}

func (mc *MCPClient) GetTools(session *MCPSession) (*[]MCPTools, error) {
	tools, err := session.cs.ListTools(mc.ctx, nil)
	if err != nil {
		return nil, err
	}
	var mcpTools []MCPTools
	for _, tool := range tools.Tools {
		params := make(map[string]string)
		for k, v := range tool.InputSchema.Properties {
			// Extract meaningful schema information instead of using String()
			var schemaDesc string
			if v.Type != "" {
				schemaDesc = v.Type
			} else if len(v.Types) > 0 {
				schemaDesc = fmt.Sprintf("[%s]", strings.Join(v.Types, ", "))
			} else {
				schemaDesc = "any"
			}

			// Add additional schema details
			if v.Description != "" {
				schemaDesc += fmt.Sprintf(" (%s)", v.Description)
			}
			if v.Format != "" {
				schemaDesc += fmt.Sprintf(" format:%s", v.Format)
			}
			if len(v.Enum) > 0 {
				schemaDesc += fmt.Sprintf(" enum:%v", v.Enum)
			}

			params[k] = schemaDesc
		}
		mcpTools = append(mcpTools, MCPTools{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  params,
			Properties:  tool.InputSchema.Properties,
		})
	}
	return &mcpTools, nil
}
