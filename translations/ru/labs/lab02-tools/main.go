package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kshvakov/agent/llm"
)

// Наша "реальная" функция
func runGetServerStatus(ip string) string {
	// Mock-реализация
	if ip == "192.168.1.10" {
		return "ONLINE (Load: 0.5)"
	}
	return "OFFLINE"
}

func main() {
	// 1. Настройка клиента: LLM_API_KEY / LLM_BASE_URL / LLM_MODEL из окружения (или из .env)
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

	ctx := context.Background()

	fmt.Printf("Client ready, endpoint: %s\n", cfg.BaseURL)
	_ = client // TODO: используйте client.Complete(ctx, req) в шаге 4

	// 2. Опишите инструмент
	// tools := []llm.ToolDef{
	//     {
	//         Name:        "get_server_status",
	//         Description: "Get the status of a server by IP address",
	//         Parameters: json.RawMessage(`{
	//             "type": "object",
	//             "properties": {
	//                 "ip": {
	//                     "type": "string",
	//                     "description": "IP address of the server"
	//                 }
	//             },
	//             "required": ["ip"]
	//         }`),
	//     },
	// }

	// 3. Сформируйте запрос
	// req := llm.Request{
	//     Model: cfg.Model,
	//     Messages: []llm.Message{
	//         {Role: "user", Content: "Is server 192.168.1.10 online?"},
	//     },
	//     Tools: tools,
	// }

	_ = ctx // TODO: передайте ctx в вызов API

	// 4. Выполните запрос и проверьте ToolCalls
	// resp, err := client.Complete(ctx, req)
	// if len(resp.Choices[0].Message.ToolCalls) > 0 { ... }
}
