package service

import (
	"context"
	"fmt"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

func main() {
	cfg := openai.DefaultConfig(os.Getenv("OPENAI_API_KEY"))
	cfg.BaseURL = "https://api.groq.com/openai/v1"
	client := openai.NewClientWithConfig(cfg)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: "openai/gpt-oss-120b",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "9.9 and 9.11, which one is larger?",
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return
	}

	fmt.Println(resp.Choices[0].Message.Content)
}