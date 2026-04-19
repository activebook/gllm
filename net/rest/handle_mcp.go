package rest

import (
	"encoding/json"
	"net/http"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
)

// MCPServerResponse represents a basic MCP server entry.
type MCPServerResponse struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Allowed     bool   `json:"allowed"`
}

func handleMCP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getMCPServers(w, r)
	case http.MethodPut:
		updateAllowedMCPServers(w, r)
	default:
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	}
}

func getMCPServers(w http.ResponseWriter, r *http.Request) {
	store := data.NewMCPStore()
	servers, err := store.Load()
	if err != nil {
		sendError(w, http.StatusInternalServerError, "LOAD_ERROR", err.Error())
		return
	}

	settingsStore := data.GetSettingsStore()
	resp := make([]MCPServerResponse, 0, len(servers))

	for name, server := range servers {
		resp = append(resp, MCPServerResponse{
			Name:        name,
			Type:        server.Type,
			Description: server.Description,
			Allowed:     settingsStore.IsMCPServerAllowed(name),
		})
	}

	sendJSON(w, http.StatusOK, resp)
}

func updateAllowedMCPServers(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Names []string `json:"names"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "List of server names required")
		return
	}

	// Validate names against registry
	store := data.NewMCPStore()
	registry, err := store.Load()
	if err != nil {
		sendError(w, http.StatusInternalServerError, "LOAD_ERROR", err.Error())
		return
	}

	for _, name := range payload.Names {
		if _, exists := registry[name]; !exists {
			sendError(w, http.StatusBadRequest, "INVALID_SERVER", "Unknown MCP server: "+name)
			return
		}
	}

	settingsStore := data.GetSettingsStore()
	if err := settingsStore.SetAllowedMCPServers(payload.Names); err != nil {
		sendError(w, http.StatusInternalServerError, "SAVE_ERROR", err.Error())
		return
	}

	// Reload MCP clients in background
	mc := service.GetMCPClient()
	mc.Close()

	configStore := data.NewConfigStore()
	activeAgent := configStore.GetActiveAgent()
	if activeAgent != nil {
		service.StartMCPServer(activeAgent)
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{"allowed": payload.Names})
}

func handleMCPDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	store := data.NewMCPStore()
	mcpConfig, err := store.Load()
	if err != nil {
		sendError(w, http.StatusInternalServerError, "LOAD_ERROR", err.Error())
		return
	}

	// Use a fresh client for discovery to avoid side effects on the global one
	client := &service.MCPClient{}
	defer client.Close()

	// Initialize with LoadAll: true to fetch from all allowed servers
	err = client.Init(mcpConfig, service.MCPLoadOption{
		LoadAll:       true,
		LoadTools:     true,
		LoadResources: true,
		LoadPrompts:   true,
	})
	if err != nil {
		sendError(w, http.StatusInternalServerError, "INIT_ERROR", err.Error())
		return
	}

	sendJSON(w, http.StatusOK, client.GetAllServers())
}
