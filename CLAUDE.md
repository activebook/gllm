# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Common Tasks
- **Build**: `make build` (outputs to `dist/gllm`)
- **Run**: `make run` (runs `main.go` directly)
- **Install**: `make install` (installs to `$GOPATH/bin`)
- **Lint**: `make lint` (runs `golangci-lint` or `go vet`)
- **Tidy Dependencies**: `make deps`

### Testing
- **Run all tests**: `make test`
- **Run specific package**: `go test ./service/ -v`
- **Run specific test**: `go test ./service/ -v -run TestName`

### PR Management
- **Create PR**: `make create-pr`
- **Merge PR**: `make merge-pr PR=123`
- **Close PR**: `make close-pr PR=123`

## Architecture and Structure

`gllm` is a Go-based CLI companion for LLMs with a decoupled architecture using an internal event bus.

### Core Layers
- **Entry Point**: `main.go` initializes the application.
- **CLI (`cmd/`)**: Use Cobra-style commands. `root.go` defines the base CLI; `repl.go` handles the interactive TUI.
- **Service Layer (`service/`)**: The core engine.
    - **Models/Providers**: `model_*.go` and `provider.go` implement LLM-specific APIs.
    - **Sessions**: `session_manager.go` and `session_*.go` handle conversation history, context compression, and persistence.
    - **Orchestration**: `agent.go` (high-level profiles), `subagent.go` (delegation), and `workflow.go`.
    - **Tools**: `tools_*.go` handles tool execution; `mcptools.go` integrates Model Context Protocol.
- **Data & State (`data/`)**:
    - `config.go`: User configuration and model definitions.
    - `sharedstate.go`: A "Blackboard" system enabling communication between independent sub-agents.
    - `memory.go`: Persistence of user-specific facts across sessions.
- **UI (`internal/ui/`)**: Built using Charm Bracelet components (Bubble Tea/Lip Gloss).
- **Internal Events (`internal/event/`)**: A central event bus facilitates communication between the service layer and the UI without tight coupling.
- **Reference System**: `@` syntax for file referencing is handled in `service/atref.go`.

### Development Patterns
- **Concurrency**: Agents often run in parallel using sub-agents; pay attention to `SharedState` thread safety.
- **Interfaces**: LLM providers and tools use interfaces defined in `service/` to allow for multi-provider support.
- **Context Management**: History compression (summarization/truncation) is handled in `service/compress.go`.
