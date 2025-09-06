# gllm - Golang Command-Line LLM Companion

`gllm` is a powerful CLI tool designed to interact seamlessly with various Large Language Models (LLMs). It supports features like interactive chat, multi-turn conversations, file attachments, search integration, a command agent, multi-agent workflows, deep research, mcp services and extensive customization.

## 🚀 Features

- **Flexible Model Selection**: Easily configure and switch between different LLMs.
- **Interactive Chat Mode**: Start real-time conversations with AI models.
- **Prompt Templates & System Prompts**: Manage reusable prompts and instructions.
- **Attachment Support**: Process files, images, and URLs as part of your queries.
- **Search Integration**: Use search engines to find the latest and most relevant information.
- **PDF & Image Processing**: Supports processing of PDF documents and images with capable models.
- **Reasoning & Deep Thinking**: Generate detailed explanations, logical breakdowns, and step-by-step analysis.
- **Markdown Support**: Renders Markdown for easy-to-read formatted output.
- **Multi-turn Conversations**: Engage in multiple rounds of conversation and manage chat history.
- **Command Agent Mode**: Let LLMs plan and execute commands with your confirmation.
- **Multi-Agent Workflows**: Build and run complex workflows with multiple agents for tasks like deep research.
- **Model Context Protocol (MCP) Support**: Connect to external MCP servers to access additional tools and data sources.
- **Token Usage Tracking**: Monitor your token consumption.
- **Configuration Management**: Easily manage models, templates, system prompts, and search engines.
- **Version Control**: Easily track and update your `gllm` setup.

---

## 📌 Installation

### Homebrew (macOS)

```sh
brew tap activebook/gllm
brew install gllm
```

### Build from Source

```sh
git clone https://github.com/activebook/gllm.git
cd gllm
go build -o gllm
```

---

## 🎯 Usage

### Basic Commands

- **Ask a simple question:**
  ```sh
  gllm "What is Go?"
  ```

- **Use a specific model:**
  ```sh
  gllm "Where is the best place to visit in London?" -m gpt4o
  ```

- **Use a template for a specific task:**
  ```sh
  gllm "How to find a process and terminate it?" -p shellmate
  ```

- **Search the web:**
  ```sh
  gllm "Who is the current POTUS?" -s
  ```
  ![Search Screenshot](screenshots/search.png)

### Interactive Chat

Start an interactive chat session:

```sh
gllm chat
```

![Chat Mode Screenshot](screenshots/chatmode.png)

Within the chat, you can use various commands:

- `/help`: Show available commands.
- `/history`: View conversation history.
- `/system <prompt>`: Change the system prompt.
- `/attach <file>`: Attach a file to the conversation.
- `! <command>`: Execute a shell command.

### Multi-turn Conversations

There are two main ways to have a multi-turn conversation:

**1. Single-Line Style (using named conversations)**

You can maintain a conversation across multiple commands by assigning a name to your conversation with the `-c` flag. This is useful for scripting or when you want to continue a specific line of inquiry.

- **Start or continue a named conversation:**
  ```sh
  gllm "Who's the POTUS right now?" -c my_convo
  gllm "Tell me more about his policies." -c my_convo
  ```

**2. Chat Style (interactive session)**

For a more interactive experience, you can use the `chat` command to enter a real-time chat session.

- **Start an interactive chat session:**
  ```sh
  gllm chat
  ```
  Within the chat, the conversation history is automatically maintained.


### File Attachments

- **Summarize a text file:**
  ```sh
  gllm "Summarize this" -a report.txt
  ```

- **Analyze an image:**
  ```sh
  gllm "What is in this image?" -a image.png
  ```

- **Process a PDF document (with a capable model like Gemini):**
  ```sh
  gllm "Summarize this PDF" -a document.pdf
  ```

### Code Editing

The command agent supports diff editing for precise code modifications.

```sh
gllm "Read this file to change function name"
```

| Edit code with diff | Cancel an edit |
|---------------------|----------------|
| ![Edit Code Screenshot](screenshots/editcode.png) | ![Cancel Edit Screenshot](screenshots/editcode_cancel.png) |

### Workflows

`gllm` allows you to define and run complex workflows with multiple agents. A workflow consists of a sequence of agents, where the output of one agent serves as the input for the next. This is useful for tasks like deep research, automated code generation, and more.

#### How it Works

A workflow is defined by a series of agents, each with a specific role and configuration. There are two types of agents:

- **`master`**: A master agent orchestrates the workflow. It takes an initial prompt and its output is passed to the next agent in the sequence. A workflow must have at least one master agent.
- **`worker`**: A worker agent performs a specific task. It receives input from the previous agent, processes it, and its output is passed to the next agent.

When a workflow is executed, `gllm` processes each agent in the defined order. The output from one agent is written to a directory that becomes the input for the next agent.

#### Workflow Commands

You can manage your workflows using the `gllm workflow` command:

- **`gllm workflow list`**: List all agents in the current workflow.
- **`gllm workflow add`**: Add a new agent to the workflow.
- **`gllm workflow remove`**: Remove an agent from the workflow.
- **`gllm workflow set`**: Modify the properties of an existing agent.
- **`gllm workflow move`**: Change the order of agents in the workflow.
- **`gllm workflow info`**: Display detailed information about a specific agent.
- **`gllm workflow start`**: Execute the workflow.

#### Example: A Simple Research Workflow

Here's an example of a simple research workflow with two agents: a `planner` and a `researcher`.

1.  **Planner (master)**: This agent takes a research topic and creates a research plan.
2.  **Researcher (worker)**: This agent takes the research plan and executes it, gathering information and generating a report.

To create this workflow, you would use the `gllm workflow add` command:

```sh
# Add the planner agent
gllm workflow add --name planner --model groq-oss --role master --output "workflow/planner" --template "Create a research plan for the following topic: {{.prompt}}"

# Add the researcher agent
gllm workflow add --name researcher --model gemini-pro --role worker --input "workflow/planner" --output "workflow/researcher" --template "Execute the following research plan: {{.prompt}}"
```

To execute the workflow, you would use the `gllm workflow start` command:

```sh
gllm workflow start "The future of artificial intelligence"
```

This will start the workflow. The `planner` agent will create a research plan and save it to the `workflow/planner` directory. The `researcher` agent will then read the plan from that directory, execute the research, and save the final report to the `workflow/researcher` directory.

Here's an example of a deep research workflow in action:

| Planner | Dispatcher | Workers | Summarizer |
|---------|------------|---------|-------------|
| Designs a plan for the research.<br>![Planner Screenshot](screenshots/deepresearch_planner.png) | Dispatches sub-tasks to worker agents.<br>![Dispatcher Screenshot](screenshots/deepresearch_dispatcher.png) | Execute the sub-tasks in parallel.<br>![Workers Screenshot](screenshots/deepresearch_workers.png) | Summarizes the results from the workers to provide a final report.<br>![Summarizer Screenshot](screenshots/deepresearch_summarizer.png) |

---

## 🛠 Model Context Protocol (MCP)

`gllm` supports the Model Context Protocol (MCP), allowing you to connect to external MCP servers to access additional tools and data sources. This enables LLMs to interact with external services, databases, and APIs through standardized protocols.

### Enabling/Disabling MCP

- **Enable MCP:**
  ```sh
  gllm mcp on
  ```

- **Disable MCP:**
  ```sh
  gllm mcp off
  ```

- **Check MCP status:**
  ```sh
  gllm mcp
  ```

### Managing MCP Servers

You can add, configure, and manage MCP servers of different types:

- **Add an MCP server:**
  ```sh
  # Add a stdio-based server
  gllm mcp add --name my-server --type std --command "my-mcp-server"

  # Add an SSE-based server
  gllm mcp add --name sse-server --type sse --url "http://example.com/mcp"

  # Add an HTTP-based server
  gllm mcp add --name http-server --type http --url "http://example.com/mcp"
  ```

- **List available MCP tools:**
  ```sh
  gllm mcp list
  ```
  ![List MCP Tools Screenshot](screenshots/listmcp.png)

- **Update an MCP server:**
  ```sh
  gllm mcp set --name my-server --allow true
  ```

- **Remove an MCP server:**
  ```sh
  gllm mcp remove --name my-server
  ```

- **Export/Import MCP servers:**
  ```sh
  gllm mcp export [path]
  gllm mcp import [path]
  ```

### Using MCP in Queries

Once MCP is enabled and servers are configured, the LLM can automatically use available MCP tools during conversations:

```sh
gllm "Use the available tools to fetch the latest info of golang version"
```

![Use MCP Screenshot](screenshots/usemcp.png)

The LLM will detect relevant MCP tools and use them to enhance its responses with external data and capabilities.

---

## 🛠 Configuration

`gllm` stores its configuration in a user-specific directory. You can manage the configuration using the `config` command.

- **Show the configuration file path:**
  ```sh
  gllm config path
  ```

- **Print all configurations:**
  ```sh
  gllm config print
  ```

- **Export/Import the configuration file:**
  ```sh
  gllm config export [directory]
  gllm config import [directory]
  ```

- **Manage models, templates, system prompts, and search engines:**
  ```sh
  gllm model --help
  gllm template --help
  gllm system --help
  gllm search --help
  ```

---

## 🏗 Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.

---

*Created by Charles Liu ([@activebook](https://github.com/activebook))*
