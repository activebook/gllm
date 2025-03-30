class Gllm < Formula
    desc "`gllm` is a powerful command-line tool designed to interact seamlessly with various Large Language Models (LLMs). Configure your API keys, set your preferred models, and start chatting or executing commands effortlessly."
    homepage "https://github.com/activebook/gllm"
    url "https://github.com/activebook/gllm/archive/refs/tags/v1.0.0.tar.gz"
    sha256 "360edc47362b1dd0c82a4fd09f880f942832674cc7c45abf7978e2ad203d948c"
    license "Apache-2.0" # or whatever license you're using
  
    depends_on "go" => :build
  
    def install
      system "go", "build", *std_go_args
    end
  
    test do
      system "#{bin}/gllm", "--version"
    end
  end