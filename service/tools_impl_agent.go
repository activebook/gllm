package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/event"
)

// listMemoryToolCallImpl handles the list_memory tool call
func listMemoryToolCallImpl() (string, error) {
	if err := CheckToolPermission(ToolListMemory, nil); err != nil {
		return "", err
	}

	memories, err := data.NewMemoryStore().Load()
	if err != nil {
		return fmt.Sprintf("Error loading memories: %v", err), nil
	}

	if len(memories) == 0 {
		return "No memories saved. The user has not asked you to remember anything yet.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Current saved memories (%d items):\n\n", len(memories)))
	for i, memory := range memories {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, memory))
	}

	return result.String(), nil
}

// saveMemoryToolCallImpl handles the save_memory tool call
// Simplified design: takes complete memory content and replaces all memories
func saveMemoryToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	if err := CheckToolPermission(ToolSaveMemory, argsMap); err != nil {
		return "", err
	}

	memories, ok := (*argsMap)["memories"].(string)
	if !ok {
		return "", fmt.Errorf("memories parameter not found in arguments")
	}

	store := data.NewMemoryStore()

	// Empty string means clear all memories
	if strings.TrimSpace(memories) == "" {
		err := store.Clear()
		if err != nil {
			return fmt.Sprintf("Error clearing memories: %v", err), nil
		}
		return "Successfully cleared all memories", nil
	}

	// Calculate new memories from content
	lines := strings.Split(memories, "\n")
	var newMemories []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			memory := strings.TrimPrefix(line, "- ")
			if memory != "" {
				newMemories = append(newMemories, memory)
			}
		} else if line != "" && !strings.HasPrefix(line, "#") {
			newMemories = append(newMemories, line)
		}
	}

	// Replace all memories with new content
	err := store.Save(newMemories)
	if err != nil {
		return fmt.Sprintf("Error updating memories: %v", err), nil
	}

	// Count how many memories were saved
	savedMemories, _ := store.Load()
	return fmt.Sprintf("Successfully updated memories (%d items saved)", len(savedMemories)), nil
}

// switchAgentToolCallImpl handles the switch_agent tool call
func switchAgentToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolSwitchAgent, argsMap); err != nil {
		return "", err
	}

	name, ok := (*argsMap)["name"].(string)
	if !ok {
		return "", fmt.Errorf("agent name is required")
	}

	store := data.NewConfigStore()

	// If name is "list", return available agents
	if name == "list" {
		agents := store.GetAllAgents()
		var sb strings.Builder

		// Title
		sb.WriteString("# Available Agents\n\n")
		sb.WriteString(fmt.Sprintf("Total: %d agent(s)\n\n", len(agents)))

		var names []string
		for n := range agents {
			names = append(names, n)
		}
		sort.Strings(names)

		// List all agents with details
		for _, n := range names {
			ag := agents[n]
			sb.WriteString(formatAgentInfo(n, ag))
		}

		// Capability Glossary
		sb.WriteString("---\n\n")
		sb.WriteString("## Capability Glossary\n\n")
		sb.WriteString(GetAllCapabilitiesDescription())
		sb.WriteString("\n\n")

		// Instructions
		sb.WriteString("---\n\n")
		sb.WriteString("## Usage\n\n")
		sb.WriteString("- Use `switch_agent` with the exact agent name to hand off execution\n")

		return sb.String(), nil
	}

	// Check if agent exists
	if store.GetAgent(name) == nil {
		return fmt.Sprintf("Agent '%s' not found. Use 'list' to see available agents.", name), nil
	}

	// If already in this agent, just return message
	if store.GetActiveAgentName() == name {
		return fmt.Sprintf("You are already using agent '%s'. No need to switch.", name), nil
	}

	if !op.toolsUse.AutoApprove {
		purpose := fmt.Sprintf("switch to agent '%s'", name)
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: switch to agent %s", name), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Set active agent
	err := store.SetActiveAgent(name)
	if err != nil {
		return fmt.Sprintf("Failed to set active agent: %v", err), nil
	}

	// Set instruction for new agent
	var instruction string
	if v, ok := (*argsMap)["instruction"].(string); ok {
		instruction = v
	}

	// Signal to switch
	return fmt.Sprintf("Switching to agent '%s'...", name), SwitchAgentError{TargetAgent: name, Instruction: instruction}
}

// buildAgentToolCallImpl handles the build_agent tool call.
// It performs deterministic validation of all enum-constrained fields
// before writing the agent .md file, returning a structured corrective
// error message to the LLM on any validation failure (reflection loop).
func buildAgentToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolBuildAgent, argsMap); err != nil {
		return "", err
	}

	args := *argsMap

	// ── Extract required string fields ──────────────────────────────────────
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	think, _ := args["think"].(string)
	systemPrompt, _ := args["system_prompt"].(string)

	if strings.TrimSpace(name) == "" {
		return "Error: 'name' is required.", nil
	}
	if strings.TrimSpace(systemPrompt) == "" {
		return "Error: 'system_prompt' is required.", nil
	}

	// ── Validate think level ─────────────────────────────────────────────────
	validThinkLevels := map[string]bool{"off": true, "minimal": true, "low": true, "medium": true, "high": true}
	if think == "" {
		think = "off"
	}
	if !validThinkLevels[think] {
		validKeys := make([]string, 0, len(validThinkLevels))
		for k := range validThinkLevels {
			validKeys = append(validKeys, k)
		}
		return fmt.Sprintf(
			"Error: 'think' value '%s' is invalid.\nYou should only use valid think levels as: %v",
			think, validKeys,
		), nil
	}

	// ── Validate and extract tools ───────────────────────────────────────────
	var selectedTools []string
	validEmbedTools := GetEmbeddingTools()
	validEmbedSet := make(map[string]bool, len(validEmbedTools))
	for _, t := range validEmbedTools {
		validEmbedSet[t] = true
	}

	toolsRaw, _ := args["tools"].([]interface{})
	var invalidTools []string
	for _, v := range toolsRaw {
		t, ok := v.(string)
		if !ok {
			continue
		}
		if !validEmbedSet[t] {
			invalidTools = append(invalidTools, t)
		} else {
			selectedTools = append(selectedTools, t)
		}
	}
	if len(invalidTools) > 0 {
		validFeatureTools := GetAllFeatureInjectedTools()
		return fmt.Sprintf(
			"Error: The following tool names are invalid: %v\n"+
				"Valid embedding tools: %v\n"+
				"Note: Feature-injected tools (%v) must NOT be placed in 'tools'; enable their corresponding capability instead.",
			invalidTools, validEmbedTools, validFeatureTools,
		), nil
	}

	// ── Validate and extract capabilities ────────────────────────────────────
	validCaps := GetAllEmbeddingCapabilities()
	validCapsSet := make(map[string]bool, len(validCaps))
	for _, c := range validCaps {
		validCapsSet[c] = true
	}
	var selectedCaps []string
	capsRaw, _ := args["capabilities"].([]interface{})
	var invalidCaps []string
	for _, v := range capsRaw {
		c, ok := v.(string)
		if !ok {
			continue
		}
		if !validCapsSet[c] {
			invalidCaps = append(invalidCaps, c)
		} else {
			selectedCaps = append(selectedCaps, c)
		}
	}
	if len(invalidCaps) > 0 {
		return fmt.Sprintf(
			"Error: The following capability names are invalid: %v\nYou should only use valid capabilities as bellow: %v",
			invalidCaps, validCaps,
		), nil
	}

	// ── Get model from active agent ────────────────────────────────────────
	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		return "Error: No active agent found.", nil
	}
	// Use agent model as default model of the new agent
	model := activeAgent.Model.Name

	// ── Check for duplicate name ─────────────────────────────────────────────
	if existing := store.GetAgent(name); existing != nil {
		agentNames := store.GetAgentNames()
		return fmt.Sprintf(
			"Error: Agent name '%s' is already taken. The following names are unavailable, choose a name that is not in this list: %v",
			name, agentNames,
		), nil
	}

	// ── optional max_recursions ──────────────────────────────────────────────
	maxRecursions := 50
	if v, ok := args["max_recursions"]; ok {
		switch mv := v.(type) {
		case float64:
			maxRecursions = int(mv)
		case int:
			maxRecursions = mv
		}
	}

	// ── Confirm before writing ───────────────────────────────────────────────

	if !op.toolsUse.AutoApprove {
		purpose := fmt.Sprintf("build agent '%s' with %d tools and %d capabilities", name, len(selectedTools), len(selectedCaps))
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: build agent '%s'", name), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// ── Write the agent file ─────────────────────────────────────────────────
	agentConfig := &data.AgentConfig{
		Name:          name,
		Description:   description,
		Model:         data.Model{Name: model},
		Tools:         selectedTools,
		Capabilities:  selectedCaps,
		Think:         think,
		MaxRecursions: maxRecursions,
		SystemPrompt:  strings.TrimSpace(systemPrompt),
	}

	if err := data.WriteAgentFile(agentConfig); err != nil {
		return fmt.Sprintf("Error writing agent file: %v", err), nil
	}

	// tell model agent is ready
	return fmt.Sprintf("Successfully created agent '%s'.", name), nil
}

// listAgentToolCallImpl handles the list_agent tool call
// Returns a formatted list of all available agents with their capabilities
func listAgentToolCallImpl() (string, error) {
	if err := CheckToolPermission(ToolListAgent, nil); err != nil {
		return "", err
	}

	store := data.NewConfigStore()
	agents := store.GetAllAgents()

	if len(agents) == 0 {
		return "No agents configured. Use 'gllm agent add' to create agents.", nil
	}

	var sb strings.Builder

	// Title
	sb.WriteString("# Available Agents\n\n")
	sb.WriteString(fmt.Sprintf("Total: %d agent(s)\n\n", len(agents)))

	// Sort agent names for consistent output
	var names []string
	for n := range agents {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		ag := agents[name]
		sb.WriteString(formatAgentInfo(name, ag))
	}

	// Capability Glossary
	sb.WriteString("---\n\n")
	sb.WriteString("## Capability Glossary\n\n")
	sb.WriteString(GetAllCapabilitiesDescription())
	sb.WriteString("\n\n")

	// Instructions
	sb.WriteString("---\n\n")
	sb.WriteString("## Usage\n\n")
	sb.WriteString("- Use `spawn_subagents` to invoke a sub-agent\n")
	sb.WriteString("- Use `switch_agent` to hand off to another agent\n")

	return sb.String(), nil
}

func formatAgentInfo(name string, ag *data.AgentConfig) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Agent: `%s`\n\n", name))
	sb.WriteString(fmt.Sprintf("- **Description:** %s\n", ag.Description))
	sb.WriteString(fmt.Sprintf("- **Model:** %s (%s)\n", ag.Model.Model, ag.Model.Provider))
	sb.WriteString(fmt.Sprintf("- **Thinking Level:** %s\n", ag.Think))

	if len(ag.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("- **Tools:** %s\n", strings.Join(ag.Tools, ", ")))
	} else {
		sb.WriteString("- **Tools:** _(none)_\n")
	}

	if len(ag.Capabilities) > 0 {
		sb.WriteString(fmt.Sprintf("- **Capabilities:** %s\n", strings.Join(ag.Capabilities, ", ")))
	} else {
		sb.WriteString("- **Capabilities:** _(none)_\n")
	}

	sb.WriteString("\n")
	return sb.String()
}

// spawnSubAgentsToolCallImpl handles the spawn_subagents tool call
// Invokes one or more sub-agents and returns progress summary
func spawnSubAgentsToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolSpawnSubAgents, argsMap); err != nil {
		return "", err
	}
	if op.executor == nil {
		return "", fmt.Errorf("sub-agent executor not initialized")
	}

	// Parse tasks array
	tasksInterface, ok := (*argsMap)["tasks"].([]interface{})
	if !ok {
		return "", fmt.Errorf("tasks parameter is required and must be an array")
	}

	if len(tasksInterface) == 0 {
		return "No tasks provided. Please specify at least one task.", nil
	}

	if !op.toolsUse.AutoApprove {
		// Build brief description of tasks
		var taskDesc strings.Builder
		for i, task := range tasksInterface {
			if i > 0 {
				taskDesc.WriteString("\n")
			}
			if tm, ok := task.(map[string]interface{}); ok {
				taskKey, _ := tm["task_key"].(string)
				agentName, _ := tm["agent_name"].(string)
				taskDesc.WriteString(fmt.Sprintf("- Task %d: %s [Agent: %s]", i+1, taskKey, agentName))
			}
		}
		if op.interaction != nil {
			op.interaction.RequestConfirm(taskDesc.String(), op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
			return "Operation cancelled by user: spawn sub-agents", UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Convert tasks to SubAgentTask structs
	var tasks []*SubAgentTask
	for i, taskInterface := range tasksInterface {
		taskMap, ok := taskInterface.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("task at index %d is not a valid object", i)
		}

		agentName, ok := taskMap["agent_name"].(string)
		if !ok || agentName == "" {
			return "", fmt.Errorf("task at index %d missing required 'agent_name' field", i)
		}

		instruction, ok := taskMap["instruction"].(string)
		if !ok || instruction == "" {
			return "", fmt.Errorf("task at index %d missing required 'instruction' field", i)
		}

		taskKey, ok := taskMap["task_key"].(string)
		if !ok || taskKey == "" {
			return "", fmt.Errorf("task at index %d missing required 'task_key' field", i)
		}

		// Parse optional input_keys
		var inputKeys []string
		if keysInterface, ok := taskMap["input_keys"].([]interface{}); ok {
			for _, k := range keysInterface {
				if keyStr, ok := k.(string); ok {
					inputKeys = append(inputKeys, keyStr)
				}
			}
		}

		tasks = append(tasks, &SubAgentTask{
			CallerAgentName: op.agentName,
			AgentName:       agentName,
			Instruction:     instruction,
			TaskKey:         taskKey,
			InputKeys:       inputKeys,
		})
	}

	// Dispatch tasks concurrently via the actor model
	responses, err := op.executor.Dispatch(tasks)
	if err != nil {
		return "", fmt.Errorf("failed to dispatch sub-agents: %v", err)
	}

	// Return formatted summary
	return op.executor.FormatSummary(responses), nil
}

// getStateToolCallImpl handles the get_state tool call
// Retrieves a value from SharedState
func getStateToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolGetState, argsMap); err != nil {
		return "", err
	}
	if op.sharedState == nil {
		return "", fmt.Errorf("shared state not initialized")
	}

	key, ok := (*argsMap)["key"].(string)
	if !ok || key == "" {
		return "", fmt.Errorf("key parameter is required")
	}

	// Get metadata for context
	meta := op.sharedState.GetMetadata(key)
	if meta == nil {
		return fmt.Sprintf("Key '%s' not found in SharedState. Use list_state to see available keys.", key), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Key: %s\n", key))
	result.WriteString(fmt.Sprintf("Created by: %s\n", meta.CreatedBy))
	result.WriteString(fmt.Sprintf("Type: %s\n", meta.ContentType))
	result.WriteString(fmt.Sprintf("Size: %d bytes\n", meta.Size))

	value := op.sharedState.GetString(key)
	result.WriteString("\nValue:\n")
	result.WriteString(value)

	return result.String(), nil
}

// setStateToolCallImpl handles the set_state tool call
// Stores a value in SharedState
func setStateToolCallImpl(
	argsMap *map[string]interface{},
	op *OpenProcessor,
) (string, error) {
	if err := CheckToolPermission(ToolSetState, argsMap); err != nil {
		return "", err
	}
	if op.sharedState == nil {
		return "", fmt.Errorf("shared state not initialized")
	}

	key, ok := (*argsMap)["key"].(string)
	if !ok || key == "" {
		return "", fmt.Errorf("key parameter is required")
	}

	value, ok := (*argsMap)["value"]
	if !ok {
		return "", fmt.Errorf("value parameter is required")
	}

	// Check if key already exists
	existed := op.sharedState.Has(key)

	err := op.sharedState.Set(key, value, op.agentName)
	if err != nil {
		return "", fmt.Errorf("failed to set state: %v", err)
	}

	if existed {
		return fmt.Sprintf("Successfully updated key '%s' in SharedState.", key), nil
	}
	return fmt.Sprintf("Successfully created key '%s' in SharedState.", key), nil
}

// listStateToolCallImpl handles the list_state tool call
// Lists all keys and metadata in SharedState
func listStateToolCallImpl(op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolListState, nil); err != nil {
		return "", err
	}

	if op.sharedState == nil {
		return "", fmt.Errorf("shared state not initialized")
	}

	return op.sharedState.FormatList(), nil
}

// activateSkillToolCallImpl handles the activate_skill tool call.
func activateSkillToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolActivateSkill, argsMap); err != nil {
		return "", err
	}

	name, ok := (*argsMap)["name"].(string)
	if !ok {
		return "", fmt.Errorf("skill name not found in arguments")
	}

	// Get or create skill manager (singleton pattern)
	sm := GetSkillManager()
	// Activate skill: Get skill details
	skillDetails, desc, tree, err := sm.ActivateSkill(name)
	if err != nil {
		return "", err
	}

	// Check if confirmation is needed (default logic: always confirm unless AutoApprove is true)
	if !op.toolsUse.AutoApprove {
		description := "Activate Skill:\n" + name + "\n\nDescription:\n" + desc + "\n\nResources:\n" + tree
		if op.interaction != nil {
			op.interaction.RequestConfirm(description, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: activate skill %s", name), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	return skillDetails, nil
}

// askUserToolCallImpl handles the ask_user tool call.
func askUserToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolAskUser, argsMap); err != nil {
		return "", err
	}

	question, _ := (*argsMap)["question"].(string)
	qType, _ := (*argsMap)["question_type"].(string)

	var options []string
	if rawOpts, ok := (*argsMap)["options"].([]interface{}); ok {
		for _, o := range rawOpts {
			if s, ok := o.(string); ok {
				options = append(options, s)
			}
		}
	}
	placeholder, _ := (*argsMap)["placeholder"].(string)

	req := event.AskUserRequest{
		Question:     question,
		QuestionType: qType,
		Options:      options,
		Placeholder:  placeholder,
	}

	resp, err := op.interaction.RequestAskUser(req)
	if err != nil {
		return "", err
	}

	// Encode answer back to the model
	out, _ := json.Marshal(resp)
	return string(out), nil
}

// enterPlanModeToolCallImpl handles the enter_plan_mode tool call.
func enterPlanModeToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolEnterPlanMode, argsMap); err != nil {
		return "", err
	}

	// Check if already in plan mode
	if data.GetPlanModeInSession() {
		return "Already in Plan Mode. Current session was already in Plan Mode.", nil
	}

	// Request user confirmation before entering Plan Mode
	if !op.toolsUse.AutoApprove {
		// Get purpose (required parameter)
		purpose, ok := (*argsMap)["purpose"].(string)
		if !ok || purpose == "" {
			return "", fmt.Errorf("purpose is required")
		}
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
			return "Operation cancelled by user: User denied entering Plan Mode.", UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Switch to plan mode
	data.SetPlanModeInSession(true)
	// Notify UI to update banner
	event.GetBus().Session <- event.SessionModeEvent{Mode: 1}

	return "Successfully switched to Plan Mode. You can now use read-only tools to research and plan. Use exit_plan_mode when you're ready to execute.", nil
}

// exitPlanModeToolCallImpl handles the exit_plan_mode tool call.
func exitPlanModeToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolExitPlanMode, argsMap); err != nil {
		return "", err
	}

	// If auto approve, we still notify but we just go directly
	if !op.toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = "exit Plan Mode and enter normal execution mode"
		}
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
			return "Operation cancelled by user: User denied exiting Plan Mode.", UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Directly mutate session state — agent runs outside RunChatInput,
	// so SendEvent is a no-op here. The next NewChatInputModel call will
	// read the updated state and hide the banner automatically.
	data.SetPlanModeInSession(false)
	// Best-effort: if RunChatInput somehow is running concurrently, update banner.
	event.GetBus().Session <- event.SessionModeEvent{Mode: 0}

	return "Successfully exited Plan Mode. Current session is now in normal execution mode.", nil
}
