## Purpose
`gllm` is a Go-based CLI companion for LLMs featuring multi-provider support (OpenAI, Anthropic, Gemini), MCP integration, and complex agentic workflows with context compression.

## Key Constraints
- **Idiomatic Go**: Adhere strictly to "Effective Go" and standard `gofmt` formatting.
- **Error Handling**: Never ignore errors with `_`; use error wrapping (`fmt.Errorf` with `%w`) or sentinel errors.
- **Encapsulation**: Use `internal/` packages to enforce boundaries; keep files under 500 lines.
- **Thread Safety**: Ensure concurrent access to `SharedState` and `SessionManager` is protected.
- **UI Decoupling**: Service layer logic must communicate with the TUI via the internal event bus (`internal/event/`).

## Workflow
1. **Dependency Management**: Run `make deps` (or `go mod tidy`) after adding imports.
2. **Development**: Implement logic in `service/` and CLI interface in `cmd/`.
3. **Verification**: Run `make lint` and `make test` before proposing any changes.
4. **Context References**: Use `@` syntax (handled by `service/atref.go`) when referencing local files in prompts.
5. **Commits**: Use Conventional Commits (e.g., `feat:`, `fix:`, `refactor:`).

## Agent Rules
- **Non-Destructive**: Never modify user configuration in `gllm.yaml` without explicit permission.
- **No Direct Main**: Propose changes via PR commands (`make create-pr`) rather than direct commits.
- **Confirmation**: Always ask before executing shell commands or file deletions unless "Yolo Mode" is explicitly active.

## Commands
- `make build`: Build binary to `dist/gllm`.
- `make test`: Run all tests.
- `make lint`: Run `golangci-lint` or `go vet`.
- `go test ./service/ -v -run <TestName>`: Run specific service tests.
- `make run`: Execute `main.go` directly.

## Architecture
- **cmd/**: CLI definition using Cobra; `repl.go` manages the interactive TUI.
- **service/**: Core engine including `provider.go` (LLM APIs), `session_manager.go` (history), and `mcptools.go` (MCP).
- **data/**: Persistence layer for configuration, user memory, and the `SharedState` blackboard.
- **internal/ui/**: TUI components built with Bubble Tea and Lip Gloss.
- **internal/event/**: Central event bus for decoupled Service-to-UI communication.