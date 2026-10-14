package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/kshvakov/agent/llm"
)

func main() {
	// 1. Client setup: LLM_API_KEY / LLM_BASE_URL / LLM_MODEL from the environment (or a sourced .env)
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
	_ = client // TODO: use client.Complete(ctx, req) in step 4

	// 2. Initialize message history
	var messages []llm.Message // TODO: Initialize message history (start with a system prompt)
	_ = messages

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

		// 3. Add user message to history
		// messages = append(messages, llm.Message{Role: "user", Content: input})

		// 4. Call API
		// req := llm.Request{
		//     Model:    cfg.Model,
		//     Messages: messages, // All history!
		// }
		// resp, err := client.Complete(ctx, req)

		// 5. Handle response & add assistant message to history
		// answer := resp.Choices[0].Message.Content
		// fmt.Println("AI:", answer)
		// messages = append(messages, llm.Message{Role: "assistant", Content: answer})
		_ = ctx
	}
}
