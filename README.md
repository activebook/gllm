# gllm - Golang Command-Line LLM Companion

`gllm` is a powerful command-line tool designed to interact seamlessly with various Large Language Models (LLMs). Configure your API keys, set your preferred models, and start chatting or executing commands effortlessly.

## 🚀 Features

- **Flexible Model Selection**: Easily configure and switch between different LLMs.
- **Interactive Chat Mode**: Start real-time conversations with AI models.
- **Prompt Templates & System Prompts**: Manage reusable prompts and instructions.
- **Attachment Support**: Process files and images as part of queries.
- **Configuration Management**: Customize model behavior and settings.
- **Version Control**: Easily track and update your setup.

---

## 📌 Installation

```sh
# Install via package manager (if available)
brew tap activebook/gllm
brew install gllm

# Or manually build from source
git clone https://github.com/activebook/gllm.git
cd gllm
go build -o gllm
```

---

## 🎯 Usage

### 🔹 Basic Commands

```sh
gllm "What is Go?"               # Default model & system prompt
gllm "Summarize this" -a report.txt  # Use file as input
gllm "Translate into English" -a image1.jpg  # Use image as input and vision model
gllm "Where is the best place to visit in London?" -m @gpt4o # Switch model
gllm "How to find a process and terminate it?" -t @shellmate  # Use shellmate prompt to specific shell question
```

### 🔹 Interactive Chat (*In Future Edition*)

```sh
gllm chat                         # Start chat with defaults
gllm chat -m gpt4o                # Start chat with a specific model
gllm chat --use-prompt coder      # Use a named system prompt
gllm chat --load my_session       # Load a saved chat session
```

### 🔹 Prompt Templates

```sh
gllm --template @coder              # Use predefined coder prompt
gllm "Act as shell" --system-prompt "You are a Linux shell..."
gllm --system-prompt @shell-assistant --template @shellmate
```

### 🔹 Configuration Management

```sh
gllm config path     # Show config file location
gllm config show     # Display loaded configurations
```

### 🔹 Model Management

```sh
gllm model list                          # List available models
gllm model add --name gpt4 --key $API_KEY --model gpt-4o --temp 0.7
gllm model default gpt4                   # Set default model
```

### 🔹 Template & System Prompt Management

```sh
gllm template list                        # List available templates
gllm template add coder "You are an expert Go programmer..."
gllm system add --name coder --content "You are an expert Go programmer..."
gllm system default coder                 # Set default system prompt
```

### 🔹 Version Information

```sh
gllm version
gllm --version
```

---

## 🛠 Configuration

By default, `gllm` stores configurations in a user-specific directory. Use the `config` commands to manage settings.

```yaml
default:
  model: gpt4
  system_prompt: coder
models:
  - name: gpt4
    endpoint: "https://api.openai.com"
    key: "$OPENAI_KEY"
    model: "gpt-4o"
    temperature: 0.7
```

---

### 💡 Why gllm?

- Simplifies interaction with LLMs via CLI.
- Supports multiple models and configurations.
- Powerful customization with templates and prompts.
- Works with text, code, and image-based queries.

Start using `gllm` today and supercharge your command-line AI experience! 🚀

---

## 🏗 Contributing

@xinasuka {
  @github: https://github.com/activebook
  @website: https://activebook.github.io
}

---