package main

import (
	"context"
	"os"

	"github.com/sashabaranov/go-openai"
)

// Наша "реальная" функция
func runGetServerStatus(ip string) string {
	// Mock implementation
	if ip == "192.168.1.10" {
		return "ONLINE (Load: 0.5)"
	}
	return "OFFLINE"
}

func main() {
	// 1. Настройка клиента
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

	// 2. Опишите инструмент
	// tools := []openai.Tool{ ... }

	// 3. Сформируйте запрос
	// req := openai.ChatCompletionRequest{
	//     Model: "gpt-4o-mini", // Или имя вашей локальной модели
	//     Messages: []openai.ChatCompletionMessage{
	//         {Role: openai.ChatMessageRoleUser, Content: "Is server 192.168.1.10 online?"},
	//     },
	//     Tools: tools,
	// }

	ctx := context.Background()
	_ = ctx
	_ = client
	_ = runGetServerStatus

	// 4. Выполните запрос и проверьте ToolCalls
	// resp, _ := client.CreateChatCompletion(ctx, req)
	// if len(resp.Choices[0].Message.ToolCalls) > 0 { ... }
}
