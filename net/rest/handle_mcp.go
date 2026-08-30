package rest

import (
	"encoding/json"
	"net/http"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
)

// MCPServerResponse represents a basic MCP server entry.
type MCPServerResponse struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Allowed     bool              `json:"allowed"`
	Tools       []MCPToolResponse `json:"tools"`
}

type MCPToolResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
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
	// 1. Load static server configurations (including blocked ones)
	store := data.NewMCPStore()
	staticServers, err := store.Load()
	if err != nil {
		sendError(w, http.StatusInternalServerError, "LOAD_ERROR", err.Error())
		return
	}

	// 2. Get live servers from the shared client (contains loaded tools)
	mc := service.GetMCPClient()
	liveServers := mc.GetAllServers()

	// Map live servers by name for fast lookup
	liveMap := make(map[string]*service.MCPServer)
	for _, ls := range liveServers {
		liveMap[ls.Name] = ls
	}

	settingsStore := data.GetSettingsStore()
	resp := make([]MCPServerResponse, 0, len(staticServers))

	// 3. Join static config with live state
	for name, server := range staticServers {
		allowed := settingsStore.IsMCPServerAllowed(name)
		var tools []MCPToolResponse

		// If server is allowed and has live tools, populate them
		if live, exists := liveMap[name]; exists && live.Tools != nil {
			for _, t := range *live.Tools {
				tools = append(tools, MCPToolResponse{
					Name:        t.Name,
					Description: t.Description,
				})
			}
		}

		resp = append(resp, MCPServerResponse{
			Name:        name,
			Type:        server.Type,
			Description: server.Description,
			Allowed:     allowed,
			Tools:       tools,
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
