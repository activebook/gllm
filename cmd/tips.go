package cmd

import (
	"math/rand"
	"time"
)

var replTips = []string{
	"Use @path or @file.go to include files in your prompt.",
	"Type !bash or !ls to execute local shell commands.",
	"Press Shift+Tab to toggle between Normal, Plan, and YOLO modes.",
	"Type /help to see all available commands and shortcuts.",
	"Use /editor to open your default editor for multi-line input.",
	"Use /history to view the current session's history in a pager.",
	"Type /clear to wipe the current session's history.",
	"Press Ctrl+V to paste an image from your clipboard.",
	"Use /attach <path> to manually add files or URLs to context.",
	"Type /copy to copy the last assistant response to clipboard.",
	"Use /yolo to let the agent run commands without confirmation.",
	"Use /plan to let the agent think and propose a plan first.",
	"Type /rename to auto-generate a title for your session.",
	"Use /compress to summarize context when it gets too long.",
	"Use /skills to manage and activate specialized agent skills.",
	"Type /mcp to manage and connect to MCP servers.",
	"Use /tools to enable or disable specific embedding tools.",
	"Type /agent to switch to a different agent persona.",
}

// getRandomTips returns n random tips from the replTips slice.
func getRandomTips(n int) []string {
	if n <= 0 {
		return nil
	}
	
	// Create a copy to shuffle
	tips := make([]string, len(replTips))
	copy(tips, replTips)
	
	// Initialize random source
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	// Shuffle
	r.Shuffle(len(tips), func(i, j int) {
		tips[i], tips[j] = tips[j], tips[i]
	})
	
	// Return up to n tips
	if n > len(tips) {
		n = len(tips)
	}
	return tips[:n]
}
