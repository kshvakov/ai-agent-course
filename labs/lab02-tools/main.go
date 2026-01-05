package main

import (
	"context"
	"os"

	"github.com/sashabaranov/go-openai"
)

// Our "real" function
func runGetServerStatus(ip string) string {
	// Mock implementation
	if ip == "192.168.1.10" {
		return "ONLINE (Load: 0.5)"
	}
	return "OFFLINE"
}

func main() {
	// 1. Client setup
	token := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if token == "" {
		token = "dummy"
	}
	config := openai.DefaultConfig(token)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)

	// 2. Describe the tool
	// tools := []openai.Tool{ ... }

	// 3. Form the request
	// req := openai.ChatCompletionRequest{
	//     Model: openai.GPT3Dot5Turbo, // Or your local model name
	//     Messages: []openai.ChatCompletionMessage{
	//         {Role: openai.ChatMessageRoleUser, Content: "Is server 192.168.1.10 online?"},
	//     },
	//     Tools: tools,
	// }

	ctx := context.Background()
	_ = ctx
	_ = client
	_ = runGetServerStatus

	// 4. Execute request and check ToolCalls
	// resp, _ := client.CreateChatCompletion(ctx, req)
	// if len(resp.Choices[0].Message.ToolCalls) > 0 { ... }
}
