package service

const (
	CapabilityMCPServers  = "mcp_servers"
	CapabilityAgentSkills = "agent_skills"
	CapabilityTokenUsage  = "token_usage"
	CapabilityMarkdown    = "markdown_output"
	CapabilitySubAgents   = "sub_agents"
)

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
