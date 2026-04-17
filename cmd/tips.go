package cmd

import (
	"math/rand"
	"time"
)

var replTips = []string{
	"Use '@path/to/folder/' to include an entire directory's context in your prompt.",
	"Enable '/plan' mode for complex refactoring; the agent will outline its strategy before modifying files.",
	"Use '/yolo' mode to let the agent autonomously execute shell commands and file edits.",
	"Connect to local databases or tools using the Model Context Protocol via the '/mcp' command.",
	"Write complex, multi-line prompts in your favorite terminal editor (Vim/Nano) using '/editor'.",
	"When a session gets too long and hits token limits, use '/compress' to intelligently summarize the context.",
	"Paste images directly from your clipboard with 'Ctrl+V' to let vision models analyze screenshots.",
	"Create custom personas with specific system prompts and toolsets using '/agent'.",
	"Install and activate specialized workflows using '/skills' to extend the agent's capabilities.",
	"Seamlessly switch between OpenAI, Anthropic, and Gemini mid-session using the '/model' command.",
	"Sub-agents can communicate and share data using the persistent SharedState blackboard.",
	"Run 'gllm serve' to launch the local web interface for a rich, browser-based chat experience.",
	"Install gllm Companion VSCode extension to bring changes into VSCode as native inline diffs, and enriches sessions.",
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
