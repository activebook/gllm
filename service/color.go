package service

import (
	"github.com/muesli/termenv"
)

var (
	// Status/Message colors
	errorColor   string
	successColor string
	warnColor    string
	infoColor    string
	debugColor   string

	// UI element colors
	cyanColor   string
	bbColor     string
	hiBlueColor string
	dimColor    string
	greyColor   string
	purpleColor string
	yellowColor string

	// Emphasis colors
	boldColor      string
	underlineColor string

	// Reset
	resetColor string

	// Agent-specific colors
	reasoningColor   string
	inReasoningColor string
	inCallingColor   string

	// Thinking level colors
	reasoningColorOff  string
	reasoningColorLow  string
	reasoningColorMed  string
	reasoningColorHigh string

	// Task completion color
	taskCompleteColor string

	// Workflow colors
	workflowColor      string
	agentRoleColor     string
	modelColor         string
	directoryColor     string
	promptColor        string
	booleanTrueColor   string
	booleanFalseColor  string
	workflowResetColor string

	// Role colors for logs/serialization (semantic naming)
	roleSystemColor    string
	roleUserColor      string
	roleAssistantColor string
	toolCallColor      string
	toolResponseColor  string
	mediaColor         string
)

const (
	// Hex codes for consistency across components
	hexRoleSystem    = "#FFA726" // Amber
	hexRoleUser      = "#66BB6A" // Muted Green
	hexRoleAssistant = "#42A5F5" // Soft Blue
	hexToolCall      = "#26C6DA" // Muted Cyan (consistent with agent.go title)
	hexToolResponse  = "#9575CD" // Muted Purple
	hexMedia         = "#26A69A" // Teal
)

func init() {
	setupColors()
}

func setupColors() {
	p := termenv.ColorProfile()
	resetColor = "\033[0m"

	// Helper to get full ANSI sequence for foreground color
	fgSeq := func(hex string) string {
		c := p.Color(hex)
		if c == nil {
			return ""
		}
		return "\033[" + c.Sequence(false) + "m"
	}

	if p == termenv.TrueColor {
		errorColor = fgSeq("#FF4500")   // Orangered
		successColor = fgSeq("#89D184") // MediumSeaGreen (Less vibrant)
		warnColor = fgSeq("#FF8C00")    // DarkOrange
		infoColor = fgSeq("#5F9EA0")    // CadetBlue (Less vibrant)
		debugColor = fgSeq("#1E90FF")   // DodgerBlue

		cyanColor = fgSeq("#00FFFF")
		bbColor = fgSeq("#696969")     // DimGray
		hiBlueColor = fgSeq("#00BFFF") // DeepSkyBlue
		dimColor = "\033[2m"
		greyColor = fgSeq("#808080")
		purpleColor = fgSeq("#9370DB") // MediumPurple
		yellowColor = fgSeq("#FFD700") // Gold

		// Task completion color (Medium Sea Green, matching successColor)
		// taskCompleteColor = fgSeq("#3CB371")
		// Task completion color (Spring Green, matching switchOnColor)
		taskCompleteColor = fgSeq("#89D184")

		// Role colors (Muted & Professional)
		roleSystemColor = fgSeq(hexRoleSystem)
		roleUserColor = fgSeq(hexRoleUser)
		roleAssistantColor = fgSeq(hexRoleAssistant)
		toolCallColor = fgSeq(hexToolCall)
		toolResponseColor = fgSeq(hexToolResponse)
		mediaColor = fgSeq(hexMedia)
	} else if p >= termenv.ANSI256 {
		errorColor = "\033[31m"
		successColor = "\033[32m"
		warnColor = "\033[38;5;208m"
		infoColor = "\033[36m"
		debugColor = "\033[34m"

		cyanColor = "\033[36m"
		bbColor = "\033[90m"
		hiBlueColor = "\033[94m"
		dimColor = "\033[2m"
		greyColor = "\033[38;5;240m"
		purpleColor = "\033[38;5;141m"
		yellowColor = "\033[33m"

		// Task completion color (matching switchOnColor)
		taskCompleteColor = "\033[38;5;82m"

		roleSystemColor = "\033[38;5;214m"   // Orange/Gold
		roleUserColor = "\033[38;5;71m"      // Greenish
		roleAssistantColor = "\033[38;5;75m" // Blueish
		toolCallColor = "\033[38;5;80m"      // Cyan-ish
		toolResponseColor = "\033[38;5;141m" // Purple-ish
		mediaColor = "\033[38;5;37m"         // Teal/Cyan
	} else {
		errorColor = "\033[31m"
		successColor = "\033[32m"
		warnColor = "\033[33m"
		infoColor = "\033[36m"
		debugColor = "\033[34m"

		cyanColor = "\033[36m"
		bbColor = "\033[90m"
		hiBlueColor = "\033[94m"
		dimColor = "\033[2m"
		greyColor = "\033[90m"
		purpleColor = "\033[35m"
		yellowColor = "\033[33m"

		// Task completion color (matching switchOnColor)
		taskCompleteColor = "\033[92m"

		roleSystemColor = "\033[33m"    // Yellow
		roleUserColor = "\033[32m"      // Green
		roleAssistantColor = "\033[34m" // Blue
		toolCallColor = "\033[35m"      // Magenta
		toolResponseColor = "\033[35m"  // Magenta
		mediaColor = "\033[36m"         // Cyan
	}

	boldColor = "\033[1m"
	underlineColor = "\033[4m"

	// Agent-specific colors (fixed ANSI)
	reasoningColor = "\033[32m"   // Green
	inReasoningColor = "\033[90m" // Bright Black
	inCallingColor = "\033[36m"   // Cyan

	// Thinking level colors
	reasoningColorOff = "\033[90m"  // Bright Black (dim/off)
	reasoningColorLow = "\033[91m"  // Bright Red
	reasoningColorMed = "\033[93m"  // Bright Yellow
	reasoningColorHigh = "\033[92m" // Bright Green

	// Workflow colors (fixed ANSI)
	workflowColor = "\033[35m"     // Magenta
	agentRoleColor = "\033[36m"    // Cyan
	modelColor = "\033[92m"        // Bright Green
	directoryColor = "\033[93m"    // Yellow
	promptColor = "\033[96m"       // Cyan
	booleanTrueColor = "\033[92m"  // Bright Green
	booleanFalseColor = "\033[90m" // Bright Black
	workflowResetColor = "\033[0m" // Reset
}
