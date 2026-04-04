package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/activebook/gllm/data"
	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/openai/openai-go/v3"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"google.golang.org/genai"
)

// InstructionSystemPrompt is the system prompt for GLLM.md generation.
// Modeled after Claude Code's actual /init philosophy: anti-boilerplate,
// evidence-only, omit-by-default.
const InstructionSystemPrompt = `You are generating a GLLM.md file injected into an AI agent's system prompt every session.

Your goal: encode what the agent needs to work effectively in THIS specific context — which may be a software project, a writing workflow, a research task, a business process, or anything else.

RULES:
- Output raw Markdown only. No preamble, no code fences.
- Only write sections for which you have concrete evidence in the provided context.
- Never write generic advice ("be helpful", "check for errors"). Every line must be specific to this project.
- Omit any section entirely if you lack evidence for it. Shorter is always better.
- Under 200 lines total.

DETECT the nature of this project from context, then emit only the relevant sections:

For ANY project:
## Purpose          — what this project/workspace is for, in 2 sentences max
## Key Constraints  — non-obvious rules the agent must always follow here
## Workflow         — how work gets done: steps, order, conventions the agent must respect
## Agent Rules      — what requires user confirmation; what the agent must never assume

For SOFTWARE projects (only if code is present):
## Commands         — exact build/test/lint/run invocations
## Architecture     — non-obvious module boundaries affecting where new code goes

For WRITING/CONTENT projects (only if docs/content is the primary artifact):
## Voice & Style    — tone, terminology, audience, formatting rules specific to this work
## Output Format    — expected structure of deliverables

For RESEARCH/DATA projects:
## Domain Context   — key concepts, terminology, data sources the agent should know
## Methodology      — how analysis/research is conducted here

If updating an existing GLLM.md: improve specificity, remove anything generic, preserve what's accurate.`

// excludedDirs is the set of directories to skip during project context scanning.
// Organized by category: these are either auto-generated, dependency stores,
// caches, or binary artifacts that add noise and no signal for the LLM.
var excludedDirs = map[string]bool{
	// Version control
	".git": true,
	".svn": true,
	".hg":  true,

	// Dependency directories (package managers)
	"node_modules":     true,
	"vendor":           true, // Go, PHP, Ruby
	"bower_components": true,
	"jspm_packages":    true,
	".pnpm-store":      true,

	// Build & dist outputs
	"dist":             true,
	"build":            true,
	"out":              true,
	"output":           true,
	"target":           true, // Rust, Java/Maven
	"bin":              true,
	"obj":              true, // .NET
	"public":           true, // often generated (Hugo, Jekyll)
	"site":             true, // MkDocs output
	"_site":            true, // Jekyll output
	"storybook-static": true,

	// Framework-specific build caches
	".next":         true, // Next.js
	".nuxt":         true, // Nuxt.js
	".svelte-kit":   true,
	".astro":        true,
	".vite":         true,
	".webpack":      true,
	".parcel-cache": true,
	".turbo":        true,
	".expo":         true, // React Native

	// Python
	"__pycache__":   true,
	".venv":         true,
	"venv":          true,
	".tox":          true,
	".mypy_cache":   true,
	".ruff_cache":   true,
	".pytest_cache": true,
	".pytype":       true,
	".pyre":         true,
	"htmlcov":       true,
	".eggs":         true,

	// Ruby
	".bundle": true,

	// Terraform / infra
	".terraform":  true,
	".serverless": true,

	// General caches & temp
	".cache":      true,
	".tmp":        true,
	"tmp":         true,
	"temp":        true,
	"coverage":    true, // test coverage reports
	".nyc_output": true,
	".jest":       true,

	// OS artifacts
	".DS_Store":       true,
	"Thumbs.db":       true,
	".Spotlight-V100": true,
	".Trashes":        true,

	// IDE / editor
	".idea":      true,
	".vscode":    true, // usually fine to skip, settings rarely help
	".worktrees": true,

	// Large generated data / locks (dirs, not files)
	".devpi": true,
}

// scanProjectContext builds richer project context for the LLM:
// depth-2 file tree + identity files + git signals + existing agent rules.
func scanProjectContext() string {
	var sb strings.Builder

	// --- Depth-2 file tree ---
	sb.WriteString("<filetree>\n")
	if entries, err := os.ReadDir("."); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if excludedDirs[name] || strings.HasPrefix(name, ".") {
				continue
			}
			if entry.IsDir() {
				sb.WriteString(fmt.Sprintf("%s/\n", name))
				if subs, err := os.ReadDir(name); err == nil {
					for _, sub := range subs {
						subName := sub.Name()
						if excludedDirs[subName] {
							continue
						}
						prefix := "  "
						if sub.IsDir() {
							sb.WriteString(fmt.Sprintf("%s%s/\n", prefix, subName))
						} else {
							sb.WriteString(fmt.Sprintf("%s%s\n", prefix, subName))
						}
					}
				}
			} else {
				sb.WriteString(fmt.Sprintf("%s\n", name))
			}
		}
	}
	sb.WriteString("</filetree>\n\n")

	// --- Identity files (ordered by signal density) ---
	// Includes existing agent rule files from other tools so we don't lose their wisdom.
	identityFiles := []string{
		"README.md", "README.rst",
		// existing agent memory files — highest value, import their rules
		"AGENTS.md", "CLAUDE.md", "GEMINI.md", ".windsurfrules",
		".github/copilot-instructions.md",
		".cursor/rules/main.md", ".cursorrules",
		// dependency / build manifests
		"go.mod", "package.json", "Cargo.toml", "pyproject.toml",
		"requirements.txt", "composer.json",
		// workflow / infra
		"Makefile", "Taskfile.yml", "justfile",
		"Dockerfile", "docker-compose.yml", "docker-compose.yaml",
		".goreleaser.yaml", ".goreleaser.yml",
		// config
		"tsconfig.json", ".eslintrc.json", ".eslintrc.js",
		"biome.json", "pyproject.toml",
	}
	shortLimits := map[string]int{
		"go.sum": 5, "package-lock.json": 5,
	}
	for _, filename := range identityFiles {
		content, err := os.ReadFile(filepath.Clean(filename))
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		limit := 120
		if l, ok := shortLimits[filename]; ok {
			limit = l
		}
		if len(lines) > limit {
			lines = lines[:limit]
		}
		trimmed := strings.TrimSpace(strings.Join(lines, "\n"))
		if trimmed == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("<file path=%q>\n%s\n</file>\n\n", filename, trimmed))
	}

	// --- Git signals: recent commits reveal real workflow & conventions ---
	appendShellOutput(&sb, "git-log", "git", "log", "--oneline", "-20")
	appendShellOutput(&sb, "git-branches", "git", "branch", "-r", "--sort=-committerdate")

	// --- CI config snippets (reveal actual build/test commands) ---
	ciFiles := []string{
		".github/workflows",
		".gitlab-ci.yml",
		".circleci/config.yml",
	}
	for _, ci := range ciFiles {
		fi, err := os.Stat(ci)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			entries, _ := os.ReadDir(ci)
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml") {
					readTruncated(&sb, filepath.Join(ci, e.Name()), 60)
				}
			}
		} else {
			readTruncated(&sb, ci, 80)
		}
	}

	// --- Existing GLLM.md (update mode) ---
	if existing, err := os.ReadFile("GLLM.md"); err == nil {
		sb.WriteString("<file path=\"GLLM.md\" note=\"already exists — update, don't replace\">\n")
		sb.WriteString(strings.TrimSpace(string(existing)))
		sb.WriteString("\n</file>\n\n")
	}

	return strings.TrimSpace(sb.String())
}

func appendShellOutput(sb *strings.Builder, tag, cmd string, args ...string) {
	out, err := exec.Command(cmd, args...).Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return
	}
	sb.WriteString(fmt.Sprintf("<%s>\n%s\n</%s>\n\n", tag, strings.TrimSpace(string(out)), tag))
}

func readTruncated(sb *strings.Builder, path string, maxLines int) {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	trimmed := strings.TrimSpace(strings.Join(lines, "\n"))
	if trimmed != "" {
		sb.WriteString(fmt.Sprintf("<file path=%q>\n%s\n</file>\n\n", path, trimmed))
	}
}

// GenerateInstructionContent scans the current working directory for project context
// and calls the active agent's provider synchronously to produce the raw Markdown
// content of GLLM.md.
//
// This function mirrors GenerateSessionName's architectural pattern exactly:
// a minimal *Agent is constructed from the config (ModelInfo + ContextManager only),
// the context is packaged as the user message, and the provider-specific sync method
// is invoked. No agentic tool loop is involved.
//
// The caller is responsible for writing the returned string to disk.
func GenerateInstructionContent(modelConfig *data.AgentConfig) (string, error) {
	ctx := scanProjectContext()
	userPrompt := fmt.Sprintf(
		"Generate GLLM.md for this project.\n\n<project_context>\n%s\n</project_context>",
		ctx,
	)

	ag := &Agent{Model: constructModelInfo(&modelConfig.Model)}
	ag.Context = NewContextManager(ag, StrategyNone)

	switch modelConfig.Model.Provider {

	case ModelProviderOpenAI:
		msgs := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(userPrompt),
		}
		return ag.GenerateOpenAISync(msgs, InstructionSystemPrompt)

	case ModelProviderAnthropic:
		msgs := []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		}
		return ag.GenerateAnthropicSync(msgs, InstructionSystemPrompt)

	case ModelProviderGemini:
		msgs := []*genai.Content{
			{
				Role:  genai.RoleUser,
				Parts: []*genai.Part{{Text: userPrompt}},
			},
		}
		return ag.GenerateGeminiSync(msgs, InstructionSystemPrompt)

	case ModelProviderOpenAICompatible:
		msgs := []*model.ChatCompletionMessage{
			{
				Role: model.ChatMessageRoleUser,
				Content: &model.ChatCompletionMessageContent{
					StringValue: volcengine.String(userPrompt),
				},
				Name: Ptr(""),
			},
		}
		return ag.GenerateOpenChatSync(msgs, InstructionSystemPrompt)

	default:
		return "", fmt.Errorf("unsupported provider for instruction generation: %s", modelConfig.Model.Provider)
	}
}
