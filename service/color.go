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
	inReasoningColor string
	inCallingColor   string
	completeColor    string

	// Workflow colors
	agentNameColor     string
	agentRoleColor     string
	modelColor         string
	directoryColor     string
	promptColor        string
	booleanTrueColor   string
	booleanFalseColor  string
	workflowResetColor string

	// Serializer colors
	messageColor      string
	systemColor       string
	userColor         string
	assistantColor    string
	modelColorSer     string // Rename to avoid conflict with workflow modelColor
	functionColor     string
	toolColor         string
	functionCallColor string
	functionRespColor string
	imageColor        string
	fileDataColor     string
	ResetColor        string
	GrayColor         string
)

func init() {
	setupColors()
}

func setupColors() {
	p := termenv.ColorProfile()
	resetColor = "\033[0m"

	if p == termenv.TrueColor {
		errorColor = p.Color("#FF4500").Sequence(false)   // Orangered
		successColor = p.Color("#32CD32").Sequence(false) // Limegreen
		warnColor = p.Color("#FF8C00").Sequence(false)    // DarkOrange
		infoColor = p.Color("#00CED1").Sequence(false)    // DarkCyan
		debugColor = p.Color("#1E90FF").Sequence(false)   // DodgerBlue

		cyanColor = p.Color("#00FFFF").Sequence(false)
		bbColor = p.Color("#696969").Sequence(false)     // DimGray
		hiBlueColor = p.Color("#00BFFF").Sequence(false) // DeepSkyBlue
		dimColor = "\033[2m"
		greyColor = p.Color("#808080").Sequence(false)
		purpleColor = p.Color("#9370DB").Sequence(false) // MediumPurple
		yellowColor = p.Color("#FFD700").Sequence(false) // Gold
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
	}

	boldColor = "\033[1m"
	underlineColor = "\033[4m"

	// Agent-specific colors (fixed ANSI)
	inReasoningColor = "\033[90m" // Bright Black
	inCallingColor = "\033[36m"   // Cyan
	completeColor = "\033[32m"    // Green

	// Workflow colors (fixed ANSI)
	agentNameColor = "\033[95m"    // Bright Magenta
	agentRoleColor = "\033[36m"    // Cyan
	modelColor = "\033[92m"        // Bright Green
	directoryColor = "\033[93m"    // Yellow
	promptColor = "\033[96m"       // Cyan
	booleanTrueColor = "\033[92m"  // Bright Green
	booleanFalseColor = "\033[90m" // Bright Black
	workflowResetColor = "\033[0m" // Reset

	// Serializer colors (TrueColor)
	messageColor = "\033[38;2;255;255;0m"      // Bright Yellow
	systemColor = "\033[38;2;255;215;0m"       // Gold
	userColor = "\033[38;2;0;255;0m"           // Lime
	assistantColor = "\033[38;2;0;0;255m"      // Blue
	modelColorSer = "\033[38;2;0;0;255m"       // Blue
	functionColor = "\033[38;2;0;255;255m"     // Cyan
	toolColor = "\033[38;2;0;255;255m"         // Cyan
	functionCallColor = "\033[38;2;255;0;255m" // Magenta
	functionRespColor = "\033[38;2;255;0;255m" // Magenta
	imageColor = "\033[38;2;255;0;0m"          // Red
	fileDataColor = "\033[38;2;255;0;0m"       // Red
	ResetColor = "\033[0m"                     // Reset
	GrayColor = "\033[38;2;128;128;128m"       // Gray
}
