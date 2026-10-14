package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kshvakov/agent/llm"
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

	ctx := context.Background()

	fmt.Printf("Client ready, endpoint: %s\n", cfg.BaseURL)
	_ = client // TODO: use client.Complete(ctx, req) in step 4

	// 2. Describe the tool
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

	// 3. Form the request
	// req := llm.Request{
	//     Model: cfg.Model,
	//     Messages: []llm.Message{
	//         {Role: "user", Content: "Is server 192.168.1.10 online?"},
	//     },
	//     Tools: tools,
	// }

	_ = ctx // TODO: pass ctx to the API call

	// 4. Execute request and check ToolCalls
	// resp, err := client.Complete(ctx, req)
	// if len(resp.Choices[0].Message.ToolCalls) > 0 { ... }
}
