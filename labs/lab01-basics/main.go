package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

func main() {
	// 1. Client setup (OpenAI or Local LLM)
	token := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")

	if token == "" {
		token = "dummy-token" // For local models, token is often not important
		fmt.Println("Warning: OPENAI_API_KEY is not set. Using dummy token.")
	}

	config := openai.DefaultConfig(token)
	if baseURL != "" {
		config.BaseURL = baseURL
		fmt.Printf("Using Custom Base URL: %s\n", baseURL)
	}

	// client := openai.NewClientWithConfig(config)
	// _ = client // TODO: remove this

	// 2. Initialize message history
	// messages := ...

	reader := bufio.NewReader(os.Stdin)
	ctx := context.Background()

	fmt.Println("DevOps Bot (Lab 01). Type 'exit' to quit.")

	for {
		fmt.Print("> ")

		var input string
		fmt.Scanln(&input) // Warning: Scanln reads only one word. Better use reader.ReadString('\n')
		_ = reader         // TODO: Use reader instead of Scanln

		if input == "exit" {
			break
		}

		// 3. Add User message to history

		// 4. Call API
		// req := openai.ChatCompletionRequest{
		//     // For local models, model name is often ignored, but it's better to specify something
		//     Model: openai.GPT3Dot5Turbo,
		//     Messages: messages,
		// }
		// resp, err := client.CreateChatCompletion(ctx, req)

		// 5. Handle response & Add Assistant message to history
		_ = ctx
	}
}
