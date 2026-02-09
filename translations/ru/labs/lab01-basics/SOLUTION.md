# Lab 01 Solution: Основы работы с LLM

## 🎯 Цель
В этой лабораторной работе мы научились основам взаимодействия с LLM: отправке запросов, получению ответов и, самое главное, **управлению контекстом**. Без сохранения контекста (истории сообщений) невозможно построить диалог.

## 📝 Разбор решения

### 1. Инициализация Клиента (Local & Cloud)
Мы добавили проверку `OPENAI_BASE_URL`. Это позволяет переключаться между облаком (OpenAI) и локальным сервером (LM Studio, Ollama, vLLM) без переписывания кода.

```go
config := openai.DefaultConfig(token)
if baseURL != "" {
    config.BaseURL = baseURL
}
client := openai.NewClientWithConfig(config)
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

	"github.com/sashabaranov/go-openai"
)

func main() {
	// Конфигурация клиента
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

	// Инициализация памяти
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
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

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: input,
		})

		req := openai.ChatCompletionRequest{
			Model:    "gpt-4o-mini", // Или "local-model", имя часто игнорируется локальными серверами
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
