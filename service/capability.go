package service

import (
	"fmt"
	"strings"
)

const (
	CapabilityMCPServers      = "mcp_servers"
	CapabilityAgentSkills     = "agent_skills"
	CapabilityAgentMemory     = "agent_memory"
	CapabilityTokenUsage      = "token_usage"
	CapabilityMarkdown        = "markdown_output"
	CapabilitySubAgents       = "sub_agents"
	CapabilityAgentDelegation = "agent_delegation"
	CapabilityWebSearch       = "web_search"
	CapabilityAutoCompression = "auto_compression"
	CapabilityPlanMode        = "plan_mode"
)

const (
	CapabilityMCPTitle          = "MCP (Model Context Protocol)"
	CapabilitySkillsTitle       = "Agent Skills"
	CapabilityMemoryTitle       = "Agent Memory"
	CapabilitySubAgentsTitle    = "Sub Agents"
	CapabilityDelegationTitle   = "Agent Delegation"
	CapabilityWebSearchTitle    = "Web Search"
	CapabilityTokenUsageTitle   = "Token Usage"
	CapabilityMarkdownTitle     = "Markdown Output"
	CapabilityAutoCompressTitle = "Auto Compression"
	CapabilityPlanModeTitle     = "Plan Mode"

	CapabilityMCPTitleHighlight          = "[MCP (Model Context Protocol)]()"
	CapabilitySkillsTitleHighlight       = "[Agent Skills]()"
	CapabilityMemoryTitleHighlight       = "[Agent Memory]()"
	CapabilitySubAgentsTitleHighlight    = "[Sub Agents]()"
	CapabilityDelegationTitleHighlight   = "[Agent Delegation]()"
	CapabilityWebSearchTitleHighlight    = "[Web Search]()"
	CapabilityTokenUsageTitleHighlight   = "[Token Usage]()"
	CapabilityMarkdownTitleHighlight     = "[Markdown Output]()"
	CapabilityAutoCompressTitleHighlight = "[Auto Compression]()"
	CapabilityPlanModeTitleHighlight     = "[Plan Mode]()"

	CapabilityMCPBody          = "enables communication with locally running MCP servers that provide additional tools and resources to extend capabilities.\nYou need to set up MCP servers specifically to use this feature."
	CapabilitySkillsBody       = "are a lightweight, open format for extending AI agent capabilities with specialized knowledge and workflows.\nAfter integrating skills, **agent** will use skills automatically."
	CapabilityMemoryBody       = "allows agents to remember important facts about you across sessions.\nFacts are used to personalize responses."
	CapabilitySubAgentsBody    = "allow you to create and manage a pool of specialized agents.\nUse to define, configure, and list agents that can be delegated tasks."
	CapabilityDelegationBody   = "allow an agent to delegate tasks or hand off control to other agents.\nUse when you need to orchestrate parallel work or fully transfer execution to a specialized agent."
	CapabilityWebSearchBody    = "enables the agent to search the web for real-time information.\nYou must configure a search engine (Google, Bing, Tavily) to use this feature."
	CapabilityTokenUsageBody   = "allows agents to track their token usage.\nThis helps you to control the cost of using the agent."
	CapabilityMarkdownBody     = "allows agents to generate final response in Markdown format.\nThis helps you to format the response in a more readable way."
	CapabilityAutoCompressBody = "automatically compresses session context using a summary when context window limits are reached.\nThis provides an infinite context window continuity with minimal detail loss."
	CapabilityPlanModeBody     = "allows agents to plan their work before executing tasks.\nUse for deepresearch, complex tasks, or collaborative work"

	CapabilityMCPDescription          = CapabilityMCPTitle + " " + CapabilityMCPBody
	CapabilitySkillsDescription       = CapabilitySkillsTitle + " " + CapabilitySkillsBody
	CapabilityMemoryDescription       = CapabilityMemoryTitle + " " + CapabilityMemoryBody
	CapabilitySubAgentsDescription    = CapabilitySubAgentsTitle + " " + CapabilitySubAgentsBody
	CapabilityDelegationDescription   = CapabilityDelegationTitle + " " + CapabilityDelegationBody
	CapabilityWebSearchDescription    = CapabilityWebSearchTitle + " " + CapabilityWebSearchBody
	CapabilityTokenUsageDescription   = CapabilityTokenUsageTitle + " " + CapabilityTokenUsageBody
	CapabilityMarkdownDescription     = CapabilityMarkdownTitle + " " + CapabilityMarkdownBody
	CapabilityAutoCompressDescription = CapabilityAutoCompressTitle + " " + CapabilityAutoCompressBody
	CapabilityPlanModeDescription     = CapabilityPlanModeTitle + " " + CapabilityPlanModeBody

	// Agent Features Description Highlight
	CapabilityMCPDescriptionHighlight          = CapabilityMCPTitleHighlight + CapabilityMCPBody
	CapabilitySkillsDescriptionHighlight       = CapabilitySkillsTitleHighlight + CapabilitySkillsBody
	CapabilityMemoryDescriptionHighlight       = CapabilityMemoryTitleHighlight + CapabilityMemoryBody
	CapabilitySubAgentsDescriptionHighlight    = CapabilitySubAgentsTitleHighlight + CapabilitySubAgentsBody
	CapabilityDelegationDescriptionHighlight   = CapabilityDelegationTitleHighlight + CapabilityDelegationBody
	CapabilityWebSearchDescriptionHighlight    = CapabilityWebSearchTitleHighlight + CapabilityWebSearchBody
	CapabilityTokenUsageDescriptionHighlight   = CapabilityTokenUsageTitleHighlight + CapabilityTokenUsageBody
	CapabilityMarkdownDescriptionHighlight     = CapabilityMarkdownTitleHighlight + CapabilityMarkdownBody
	CapabilityAutoCompressDescriptionHighlight = CapabilityAutoCompressTitleHighlight + CapabilityAutoCompressBody
	CapabilityPlanModeDescriptionHighlight     = CapabilityPlanModeTitleHighlight + CapabilityPlanModeBody
)

var (
	embeddingCapabilities = []string{
		CapabilityMCPServers,
		CapabilityAgentSkills,
		CapabilityAgentMemory,
		CapabilityTokenUsage,
		CapabilityMarkdown,
		CapabilitySubAgents,
		CapabilityAgentDelegation,
		CapabilityWebSearch,
		CapabilityAutoCompression,
		CapabilityPlanMode,
	}
)

// GetAllEmbeddingCapabilities returns all capabilities that are enabled by default.
func GetAllEmbeddingCapabilities() []string {
	return embeddingCapabilities
}

// GetAllCapabilitiesDescription returns all capabilities description.
func GetAllCapabilitiesDescription() string {
	var sb strings.Builder
	for _, cap := range embeddingCapabilities {
		desc := GetCapabilityDescription(cap)
		if desc != "" {
			desc = strings.ReplaceAll(desc, "\n", " ")
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", cap, desc))
		}
	}
	return sb.String()
}

// GetCapabilityTitle returns the title of a capability.
func GetCapabilityTitle(cap string) string {
	switch cap {
	case CapabilityMCPServers:
		return CapabilityMCPTitle
	case CapabilityAgentSkills:
		return CapabilitySkillsTitle
	case CapabilityAgentMemory:
		return CapabilityMemoryTitle
	case CapabilityTokenUsage:
		return CapabilityTokenUsageTitle
	case CapabilityMarkdown:
		return CapabilityMarkdownTitle
	case CapabilitySubAgents:
		return CapabilitySubAgentsTitle
	case CapabilityAgentDelegation:
		return CapabilityDelegationTitle
	case CapabilityWebSearch:
		return CapabilityWebSearchTitle
	case CapabilityAutoCompression:
		return CapabilityAutoCompressTitle
	case CapabilityPlanMode:
		return CapabilityPlanModeTitle
	default:
		return "Unknown"
	}
}

// GetCapabilityDescHighlight returns the description of a capability with highlight.
// This is used for the dynamic note in the capabilities switch.
func GetCapabilityDescHighlight(cap string) string {
	switch cap {
	case CapabilityMCPServers, CapabilityMCPTitle:
		return CapabilityMCPDescriptionHighlight
	case CapabilityAgentSkills, CapabilitySkillsTitle:
		return CapabilitySkillsDescriptionHighlight
	case CapabilityTokenUsage, CapabilityTokenUsageTitle:
		return CapabilityTokenUsageDescriptionHighlight
	case CapabilityMarkdown, CapabilityMarkdownTitle:
		return CapabilityMarkdownDescriptionHighlight
	case CapabilitySubAgents, CapabilitySubAgentsTitle:
		return CapabilitySubAgentsDescriptionHighlight
	case CapabilityAgentDelegation, CapabilityDelegationTitle:
		return CapabilityDelegationDescriptionHighlight
	case CapabilityAgentMemory, CapabilityMemoryTitle:
		return CapabilityMemoryDescriptionHighlight
	case CapabilityWebSearch, CapabilityWebSearchTitle:
		return CapabilityWebSearchDescriptionHighlight
	case CapabilityAutoCompression, CapabilityAutoCompressTitle:
		return CapabilityAutoCompressDescriptionHighlight
	case CapabilityPlanMode, CapabilityPlanModeTitle:
		return CapabilityPlanModeDescriptionHighlight
	default:
		return ""
	}
}

// GetCapabilityDescription returns the description of a capability.
func GetCapabilityDescription(cap string) string {
	switch cap {
	case CapabilityMCPServers, CapabilityMCPTitle:
		return CapabilityMCPDescription
	case CapabilityAgentSkills, CapabilitySkillsTitle:
		return CapabilitySkillsDescription
	case CapabilityTokenUsage, CapabilityTokenUsageTitle:
		return CapabilityTokenUsageDescription
	case CapabilityMarkdown, CapabilityMarkdownTitle:
		return CapabilityMarkdownDescription
	case CapabilitySubAgents, CapabilitySubAgentsTitle:
		return CapabilitySubAgentsDescription
	case CapabilityAgentDelegation, CapabilityDelegationTitle:
		return CapabilityDelegationDescription
	case CapabilityAgentMemory, CapabilityMemoryTitle:
		return CapabilityMemoryDescription
	case CapabilityWebSearch, CapabilityWebSearchTitle:
		return CapabilityWebSearchDescription
	case CapabilityAutoCompression, CapabilityAutoCompressTitle:
		return CapabilityAutoCompressDescription
	case CapabilityPlanMode, CapabilityPlanModeTitle:
		return CapabilityPlanModeDescription
	default:
		return ""
	}
}

/*
 * Capability Utils
 */
func isCapabilityEnabled(capabilities []string, cap string) bool {
	for _, c := range capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func enableCapability(capabilities []string, cap string) []string {
	var newCaps []string
	for _, c := range capabilities {
		if c != cap {
			newCaps = append(newCaps, c)
		}
	}
	return append(newCaps, cap)
}

func disableCapability(capabilities []string, cap string) []string {
	var newCaps []string
	for _, c := range capabilities {
		if c != cap {
			newCaps = append(newCaps, c)
		}
	}
	return newCaps
}

/*
 * MCP Servers
 */
func IsMCPServersEnabled(capabilities []string) bool {
	return isCapabilityEnabled(capabilities, CapabilityMCPServers)
}

func EnableMCPServers(capabilities []string) []string {
	return enableCapability(capabilities, CapabilityMCPServers)
}

func DisableMCPServers(capabilities []string) []string {
	return disableCapability(capabilities, CapabilityMCPServers)
}

/*
 * Agent Skills
 */
func IsAgentSkillsEnabled(capabilities []string) bool {
	return isCapabilityEnabled(capabilities, CapabilityAgentSkills)
}

func EnableAgentSkills(capabilities []string) []string {
	return enableCapability(capabilities, CapabilityAgentSkills)
}

func DisableAgentSkills(capabilities []string) []string {
	return disableCapability(capabilities, CapabilityAgentSkills)
}

/*
 * Token Usage
 */
func IsTokenUsageEnabled(capabilities []string) bool {
	return isCapabilityEnabled(capabilities, CapabilityTokenUsage)
}

func EnableTokenUsage(capabilities []string) []string {
	return enableCapability(capabilities, CapabilityTokenUsage)
}

func DisableTokenUsage(capabilities []string) []string {
	return disableCapability(capabilities, CapabilityTokenUsage)
}

/*
 * Markdown
 */
func IsMarkdownEnabled(capabilities []string) bool {
	return isCapabilityEnabled(capabilities, CapabilityMarkdown)
}

func EnableMarkdown(capabilities []string) []string {
	return enableCapability(capabilities, CapabilityMarkdown)
}

func DisableMarkdown(capabilities []string) []string {
	return disableCapability(capabilities, CapabilityMarkdown)
}

/*
 * Sub Agents
 */
func IsSubAgentsEnabled(capabilities []string) bool {
	return isCapabilityEnabled(capabilities, CapabilitySubAgents)
}

func EnableSubAgents(capabilities []string) []string {
	return enableCapability(capabilities, CapabilitySubAgents)
}

func DisableSubAgents(capabilities []string) []string {
	return disableCapability(capabilities, CapabilitySubAgents)
}

/*
 * Agent Delegation
 */
func IsAgentDelegationEnabled(capabilities []string) bool {
	return isCapabilityEnabled(capabilities, CapabilityAgentDelegation)
}

func EnableAgentDelegation(capabilities []string) []string {
	return enableCapability(capabilities, CapabilityAgentDelegation)
}

func DisableAgentDelegation(capabilities []string) []string {
	return disableCapability(capabilities, CapabilityAgentDelegation)
}

/*
 * Agent Memory
 */
func IsAgentMemoryEnabled(capabilities []string) bool {
	return isCapabilityEnabled(capabilities, CapabilityAgentMemory)
}

func EnableAgentMemory(capabilities []string) []string {
	return enableCapability(capabilities, CapabilityAgentMemory)
}

func DisableAgentMemory(capabilities []string) []string {
	return disableCapability(capabilities, CapabilityAgentMemory)
}

/*
 * Web Search
 */
func IsWebSearchEnabled(capabilities []string) bool {
	return isCapabilityEnabled(capabilities, CapabilityWebSearch)
}

func EnableWebSearch(capabilities []string) []string {
	return enableCapability(capabilities, CapabilityWebSearch)
}

func DisableWebSearch(capabilities []string) []string {
	return disableCapability(capabilities, CapabilityWebSearch)
}

/*
 * Auto Compression
 */
func IsAutoCompressionEnabled(capabilities []string) bool {
	return isCapabilityEnabled(capabilities, CapabilityAutoCompression)
}

func EnableAutoCompression(capabilities []string) []string {
	return enableCapability(capabilities, CapabilityAutoCompression)
}

func DisableAutoCompression(capabilities []string) []string {
	return disableCapability(capabilities, CapabilityAutoCompression)
}

/*
 * Plan Mode
 */
func IsPlanModeEnabled(capabilities []string) bool {
	return isCapabilityEnabled(capabilities, CapabilityPlanMode)
}

func EnablePlanMode(capabilities []string) []string {
	return enableCapability(capabilities, CapabilityPlanMode)
}

func DisablePlanMode(capabilities []string) []string {
	return disableCapability(capabilities, CapabilityPlanMode)
}
