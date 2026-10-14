package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/kshvakov/agent/llm"
)

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
	_ = client // TODO: используйте client.Complete(ctx, req) в шаге 4

	// 2. Инициализируйте историю сообщений
	var messages []llm.Message // TODO: Инициализируйте историю сообщений (начните с system prompt)
	_ = messages

	reader := bufio.NewReader(os.Stdin)
	ctx := context.Background()

	fmt.Println("DevOps Bot (Lab 01). Type 'exit' to quit.")

	for {
		fmt.Print("> ")

		var input string
		fmt.Scanln(&input) // Внимание: Scanln читает только одно слово. Лучше использовать reader.ReadString('\n')
		_ = reader         // TODO: Используйте reader вместо Scanln

		if input == "exit" {
			break
		}

		// 3. Добавьте сообщение пользователя в историю
		// messages = append(messages, llm.Message{Role: "user", Content: input})

		// 4. Вызов API
		// req := llm.Request{
		//     Model:    cfg.Model,
		//     Messages: messages, // Вся история!
		// }
		// resp, err := client.Complete(ctx, req)

		// 5. Обработайте ответ и добавьте сообщение ассистента в историю
		// answer := resp.Choices[0].Message.Content
		// fmt.Println("AI:", answer)
		// messages = append(messages, llm.Message{Role: "assistant", Content: answer})
		_ = ctx
	}
}
