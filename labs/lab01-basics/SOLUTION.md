# Lab 01 Solution: LLM Basics

## 🎯 Goal
In this lab we learned the basics of interacting with an LLM: sending requests, receiving responses, and, most importantly, **context management**. Without saving context (message history), it's impossible to build a dialogue.

## 📝 Solution Breakdown

### 1. Client Initialization (Local & Cloud)
`llm.LoadConfig()` reads `LLM_API_KEY` / `LLM_BASE_URL` / `LLM_MODEL` (exported or sourced from `.env`). This allows switching between cloud and a local server (LM Studio, Ollama, vLLM) without rewriting code.

```go
cfg, err := llm.LoadConfig()
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
client := llm.NewOpenAI(
    llm.WithAPIKey(cfg.APIKey),
    llm.WithBaseURL(cfg.BaseURL),
    llm.WithModel(cfg.Model),
)
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

	"github.com/kshvakov/agent/llm"
)

func main() {
	// Client configuration: LLM_API_KEY / LLM_BASE_URL / LLM_MODEL from the environment (or a sourced .env)
	cfg, err := llm.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	client := llm.NewOpenAI(
		llm.WithAPIKey(cfg.APIKey),
		llm.WithBaseURL(cfg.BaseURL),
		llm.WithModel(cfg.Model),
	)

	// Memory initialization
	messages := []llm.Message{
		{
			Role:    "system",
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

		messages = append(messages, llm.Message{
			Role:    "user",
			Content: input,
		})

		resp, err := client.Complete(ctx, llm.Request{
			Model:    cfg.Model, // From LLM_MODEL (default glm-5.3)
			Messages: messages,
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		answer := resp.Choices[0].Message.Content
		fmt.Println("AI:", answer)

		messages = append(messages, llm.Message{
			Role:    "assistant",
			Content: answer,
		})
	}
}
```
