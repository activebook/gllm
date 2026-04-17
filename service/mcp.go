package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http" // Retained as it's used by headerTransport
	"os"
	"os/exec" // Retained as it's used by AddStdServer
	"strings"
	"sync"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/event"
	"github.com/activebook/gllm/util"
	"github.com/charmbracelet/lipgloss"
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

type MCPTool struct {
	Name        string
	Description string
	Parameters  map[string]string
	Properties  map[string]*jsonschema.Schema // Keep origin JSON Schema
}

type MCPResource struct {
	Name        string
	Description string
	URI         string
	MIMEType    string
}

type MCPPrompt struct {
	Name        string
	Description string
	Parameters  map[string]string
}

type MCPServer struct {
	Name      string
	Allowed   bool
	Tools     *[]MCPTool
	Resources *[]MCPResource
	Prompts   *[]MCPPrompt
}

type MCPSession struct {
	name string
	cs   *mcp.ClientSession
}

type MCPClient struct {
	mu            sync.Mutex
	serverMu      map[string]*sync.Mutex // Per-server locks to prevent duplicate connections
	ctx           context.Context
	cancel        context.CancelFunc
	client        *mcp.Client
	sessions      []*MCPSession
	servers       []*MCPServer
	connected     map[string]bool
	toolToSession map[string]*MCPSession
	loaded        bool // Whether MCP is loaded already
}
type MCPLoadOption struct {
	LoadAll       bool // load all tools(allowed|blocked)
	LoadTools     bool // load tools (tools/list)
	LoadResources bool // load resources (resources/list)
	LoadPrompts   bool // load prompts (prompts/list)
}

/*
A singleton pattern for the MCP client is an excellent approach.
Since MCP functionality is independent of the LLM model,
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

// IsReady returns true if the client is initialized and has at least one tool loaded.
// It is safe to call without locking.
func (mc *MCPClient) IsReady() bool {
	// mc.mu.Lock()
	// defer mc.mu.Unlock()
	// return len(mc.toolToSession) > 0

	// when loaded is true, it means the MCP client is loaded once
	return mc.client != nil && mc.ctx != nil && mc.cancel != nil && mc.loaded
}

func (mc *MCPClient) setMCPStatus() {
	mc.mu.Lock()
	nServers := len(mc.connected)
	nTools := len(mc.toolToSession)
	mc.mu.Unlock()
	event.SendStatus(fmt.Sprintf("MCP Loaded: %d servers %d tools", nServers, nTools))
}

// PreloadAsync initializes the MCP client in the background.
func (mc *MCPClient) PreloadAsync(servers map[string]*data.MCPServer, option MCPLoadOption) {
	go func() {
		if mc.IsReady() {
			// Refresh status for the UI even if already ready
			// fmt.Println("MCP is already ready")
			mc.setMCPStatus()
			return
		}

		// Only show loading status if there are actually servers to load
		if len(servers) > 0 {
			event.SendStatus("Loading MCP servers...")
		}

		err := mc.Init(servers, option)
		if err != nil {
			if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") {
				// fmt.Println("MCP initialization aborted by /mcp switch")
				return // Aborted by /mcp switch, yield to new initialization stream
			}
			event.SendBanner(getMCPFialedBanner(err))
		}
		mc.setMCPStatus() // Show status regardless of error
	}()
}

func getMCPFialedBanner(err error) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.WarnStatusHex)).
		Bold(true)
	return style.Render(fmt.Sprintf("▲ Warning: %v", err))
}

// three types of transports supported:
// httpUrl → StreamableHTTPClientTransport
// url → SSEClientTransport
// command → StdioClientTransport
// Only want list all servers, unless loadAll is false, then only load allowed servers
func (mc *MCPClient) Init(servers map[string]*data.MCPServer, option MCPLoadOption) error {
	mc.mu.Lock()
	if mc.client == nil {
		mc.ctx, mc.cancel = context.WithCancel(context.Background())
		mc.toolToSession = make(map[string]*MCPSession)
		mc.connected = make(map[string]bool)
		mc.serverMu = make(map[string]*sync.Mutex)
		// Create a new client, with no features.
		mc.client = mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
	}

	initCtx, cancelInit := context.WithTimeout(mc.ctx, 30*time.Second)
	mc.mu.Unlock()
	defer cancelInit()

	var err error
	// Connect to each server based on its type
	for serverName, server := range servers {
		// Skip if not in allowed list (if allow list is not empty)
		if !server.Allowed && !option.LoadAll {
			continue
		}

		// Retrieve or create a per-server mutex under the global lock.
		// This prevents concurrent Init calls from spawning duplicate connections
		// to the same server (e.g. background autoload + user /mcp load racing).
		mc.mu.Lock()
		if mc.serverMu[serverName] == nil {
			mc.serverMu[serverName] = &sync.Mutex{}
		}
		srvMu := mc.serverMu[serverName]
		mc.mu.Unlock()

		// Acquire the per-server lock BEFORE checking isConnected.
		// Any concurrent goroutine attempting the same server will block here
		// and then see isConnected == true once the first goroutine finishes.
		srvMu.Lock()

		mc.mu.Lock()
		isConnected := mc.connected[serverName]
		mc.mu.Unlock()

		if isConnected {
			srvMu.Unlock()
			continue // Already connected, skip
		}

		// Connect and add session
		var session *MCPSession
		if server.Type == "sse" || server.URL != "" || server.BaseURL != "" {
			// Add SSE server
			session, err = mc.AddSseServer(initCtx, serverName, server.BaseURL, server.Headers)
		} else if server.Type == "stdio" || server.Type == "std" || server.Type == "local" || server.Command != "" {
			// Add stdio server
			dir := server.WorkDir
			if dir == "" {
				dir = server.Cwd
			}
			session, err = mc.AddStdServer(initCtx, serverName, server.Command, server.Env, dir, server.Args...)
		} else if server.Type == "http" || server.HTTPUrl != "" {
			// Add HTTP server
			session, err = mc.AddHttpServer(initCtx, serverName, server.HTTPUrl, server.Headers)
		}

		if err != nil {
			// don't continue with other servers
			err = fmt.Errorf("error loading mcp server %s: %w", serverName, err)
			srvMu.Unlock()
			break
		}

		var tools *[]MCPTool
		if option.LoadTools {
			tools, err = mc.GetTools(initCtx, session)
			if err != nil {
				err = fmt.Errorf("error loading mcp server %s: %w", serverName, err)
				srvMu.Unlock()
				break
			}
		}
		var resources *[]MCPResource
		if option.LoadResources {
			resources, _ = mc.GetResources(initCtx, session)
		}
		var prompts *[]MCPPrompt
		if option.LoadPrompts {
			prompts, _ = mc.GetPrompts(initCtx, session)
		}

		mc.mu.Lock()

		// Populate tool to session map for fast lookup
		// Bugfix: remember we load servers in parallel (/mcp load and autoload in background),
		// so we need to check for duplicates when multiple servers have the same tool name
		// or, the same server is loaded multiple times
		var filteredTools []MCPTool
		if tools != nil {
			for _, tool := range *tools {
				// Prevent shadowing built-in system tools
				if IsAvailableOpenTool(tool.Name) {
					util.LogWarnf("MCP tool %q from server %q conflicts with built-in tool, ignored\n", tool.Name, serverName)
					continue
				}

				// Prevent duplicates across different MCP servers
				if _, exists := mc.toolToSession[tool.Name]; exists {
					util.LogWarnf("Duplicate MCP tool ignored: %q (from server %q)\n", tool.Name, serverName)
					continue
				}

				mc.toolToSession[tool.Name] = session
				filteredTools = append(filteredTools, tool)
			}
		}

		mc.servers = append(mc.servers, &MCPServer{
			Name: serverName, Allowed: server.Allowed,
			Tools: &filteredTools, Prompts: prompts, Resources: resources})
		mc.connected[serverName] = true
		mc.mu.Unlock()
		srvMu.Unlock()
	}
	mc.mu.Lock()
	mc.loaded = true
	mc.mu.Unlock()
	return err
}

func (mc *MCPClient) Close() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	for _, session := range mc.sessions {
		session.cs.Close()
	}
	if mc.cancel != nil {
		mc.cancel()
		mc.cancel = nil
	}
	mc.sessions = []*MCPSession{}
	mc.servers = []*MCPServer{}
	mc.toolToSession = nil
	mc.connected = nil
	mc.client = nil
	mc.ctx = nil
	mc.loaded = false
}

func (mc *MCPClient) AddSseServer(ctx context.Context, name string, url string, headers map[string]string) (*MCPSession, error) {
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
	session, err := mc.client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	// Keep track of the session
	mcpSession := &MCPSession{name, session}
	mc.mu.Lock()
	mc.sessions = append(mc.sessions, mcpSession)
	mc.mu.Unlock()
	return mcpSession, nil
}

func (mc *MCPClient) AddHttpServer(ctx context.Context, name string, url string, headers map[string]string) (*MCPSession, error) {
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
	session, err := mc.client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	// Keep track of the session
	mcpSession := &MCPSession{name, session}
	mc.mu.Lock()
	mc.sessions = append(mc.sessions, mcpSession)
	mc.mu.Unlock()
	return mcpSession, nil
}

func (mc *MCPClient) AddStdServer(ctx context.Context, name string, cmd string, env map[string]string, cwd string, args ...string) (*MCPSession, error) {
	// IMPORTANT: WE WRAP THE COMMAND TO FILTER NOISY STDOUT (NON-JSON OUTPUT)
	// Run: gllm _mcp-filter -- cmd args...
	// "--" is a common convention in Unix-like systems to prevent arguments starting with - from being misinterpreted as flags or options.
	// Cobra handles "--" internally by stopping flag parsing when encountered and excluding it from the arguments passed to the command handler, ensuring that subsequent arguments are treated as positional regardless of leading - characters.

	// Don't use _mcp-filter for now to simplify debugging
	// transport := &mcp.CommandTransport{Command: exec.Command(cmd, args...)}

	// Construct new args: _mcp-filter, --, cmd, args...
	newArgs := []string{"_mcp-filter", "--", cmd}
	newArgs = append(newArgs, args...)
	// Use ExecutorPath which points to the current binary
	transport := &mcp.CommandTransport{Command: exec.Command(ExecutorPath, newArgs...)}

	// Set the environment variables
	transport.Command.Env = os.Environ()
	for k, v := range env {
		transport.Command.Env = append(transport.Command.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set the working directory
	transport.Command.Dir = cwd

	// Connect to the server
	session, err := mc.client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	// Keep track of the session
	mcpSession := &MCPSession{name, session}
	mc.mu.Lock()
	mc.sessions = append(mc.sessions, mcpSession)
	mc.mu.Unlock()
	return mcpSession, nil
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
			base64Data := util.GetBase64String(cc.Data)
			str := fmt.Sprintf("data:%s;base64,%s", cc.MIMEType, base64Data)
			response.Types = append(response.Types, MCPResponseImage)
			response.Contents = append(response.Contents, str)
		} else if cc, ok := c.(*mcp.AudioContent); ok {
			base64Data := util.GetBase64String(cc.Data)
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

func (mc *MCPClient) GetTools(ctx context.Context, session *MCPSession) (*[]MCPTool, error) {
	tools, err := session.cs.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	var mcpTools []MCPTool
	for _, tool := range tools.Tools {
		params := make(map[string]string)

		// Convert InputSchema from any to *jsonschema.Schema
		var schema *jsonschema.Schema
		if s, ok := tool.InputSchema.(*jsonschema.Schema); ok {
			schema = s
		} else if m, ok := tool.InputSchema.(map[string]interface{}); ok {
			// Convert map to JSON and then to Schema
			data, err := json.Marshal(m)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal input schema: %v", err)
			}
			schema = &jsonschema.Schema{}
			if err := json.Unmarshal(data, schema); err != nil {
				return nil, fmt.Errorf("failed to unmarshal input schema: %v", err)
			}
		} else {
			return nil, fmt.Errorf("unsupported InputSchema type: %T", tool.InputSchema)
		}

		for k, v := range schema.Properties {
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
		mcpTools = append(mcpTools, MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  params,
			Properties:  schema.Properties,
		})
	}
	return &mcpTools, nil
}

func (mc *MCPClient) GetResources(ctx context.Context, session *MCPSession) (*[]MCPResource, error) {
	res, err := session.cs.ListResources(ctx, nil)
	if err != nil {
		// "resources/list": Method not found
		return nil, err
	}

	var mcpResources []MCPResource
	for _, resource := range res.Resources {
		mcpResources = append(mcpResources, MCPResource{
			Name:        resource.Name,
			Description: resource.Description,
			MIMEType:    resource.MIMEType,
			URI:         resource.URI,
		})
	}
	return &mcpResources, nil
}

func (mc *MCPClient) GetPrompts(ctx context.Context, session *MCPSession) (*[]MCPPrompt, error) {
	prompts, err := session.cs.ListPrompts(ctx, nil)
	if err != nil {
		// "prompts/list": Method not found
		return nil, err
	}
	var mcpPrompts []MCPPrompt
	for _, prompt := range prompts.Prompts {
		params := make(map[string]string)
		for _, arg := range prompt.Arguments {
			params[arg.Name] = arg.Description
		}
		mcpPrompts = append(mcpPrompts, MCPPrompt{
			Name:        prompt.Name,
			Description: prompt.Description,
			Parameters:  params,
		})
	}
	return &mcpPrompts, nil
}
