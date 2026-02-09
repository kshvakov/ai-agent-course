# Lab 01 Solution: LLM Basics

## 🎯 Goal
In this lab we learned the basics of interacting with an LLM: sending requests, receiving responses, and, most importantly, **context management**. Without saving context (message history), it's impossible to build a dialogue.

## 📝 Solution Breakdown

### 1. Client Initialization (Local & Cloud)
We added a check for `OPENAI_BASE_URL`. This allows switching between cloud (OpenAI) and local server (LM Studio, Ollama, vLLM) without rewriting code.

```go
config := openai.DefaultConfig(token)
if baseURL != "" {
    config.BaseURL = baseURL
}
client := openai.NewClientWithConfig(config)
```

### 2. Memory Management (Context Loop)
An LLM "doesn't remember" previous messages. We must store history ourselves and send it entirely with each request.

### 🔍 Complete Solution Code

```go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

func main() {
	// Client configuration
	token := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")

	if token == "" {
		token = "local-token"
		fmt.Println("No API Key provided. Assuming local model usage.")
	}

	config := openai.DefaultConfig(token)
	if baseURL != "" {
		config.BaseURL = baseURL
		fmt.Printf("Connected to: %s\n", baseURL)
	}

	client := openai.NewClientWithConfig(config)

	// Memory initialization
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "You are an experienced Linux administrator. Answer briefly and to the point.",
		},
	}

	reader := bufio.NewReader(os.Stdin)
	ctx := context.Background()

	fmt.Println("DevOps Bot (Lab 01). Type 'exit' to quit.")

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "exit" {
			break
		}
		if input == "" {
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: input,
		})

		req := openai.ChatCompletionRequest{
			Model:    "gpt-4o-mini", // Or "local-model", name is often ignored by local servers
			Messages: messages,
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		answer := resp.Choices[0].Message.Content
		fmt.Println("AI:", answer)

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: answer,
		})
	}
}
```
