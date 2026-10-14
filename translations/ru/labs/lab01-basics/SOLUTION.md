# Lab 01 Solution: Основы работы с LLM

## 🎯 Цель
В этой лабораторной работе мы научились основам взаимодействия с LLM: отправке запросов, получению ответов и, самое главное, **управлению контекстом**. Без сохранения контекста (истории сообщений) невозможно построить диалог.

## 📝 Разбор решения

### 1. Инициализация Клиента (Local & Cloud)
`llm.LoadConfig()` читает `LLM_API_KEY` / `LLM_BASE_URL` / `LLM_MODEL` (экспортированные или загруженные из `.env`). Это позволяет переключаться между облаком и локальным сервером (LM Studio, Ollama, vLLM) без переписывания кода.

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

### 2. Управление Памятью (Context Loop)
LLM "не помнит" предыдущие сообщения. Мы должны сами хранить историю и отправлять её каждый раз целиком.

### 🔍 Полный код решения

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
	// Конфигурация клиента: LLM_API_KEY / LLM_BASE_URL / LLM_MODEL из окружения (или из .env)
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

	// Инициализация памяти
	messages := []llm.Message{
		{
			Role:    "system",
			Content: "Ты опытный Linux администратор. Отвечай кратко и по делу.",
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
			Model:    cfg.Model, // Из LLM_MODEL (по умолчанию glm-5.3)
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
