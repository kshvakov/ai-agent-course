package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

const (
	maxContextTokens = 4000 // Лимит контекстного окна (GPT-3.5-turbo)
	threshold80      = int(float64(maxContextTokens) * 0.8)
	threshold90      = int(float64(maxContextTokens) * 0.9)
)

func main() {
	// Настройка клиента
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

	ctx := context.Background()

	// Инициализация истории
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "Ты вежливый помощник. Помни важные детали о пользователе.",
		},
	}

	fmt.Println("=== Lab 09: Context Optimization ===")
	fmt.Println("Введите сообщения. После 10+ сообщений контекст начнет оптимизироваться.")
	fmt.Println("Попробуйте спросить о ранних сообщениях после оптимизации.\n")

	// Симуляция длинного диалога
	testMessages := []string{
		"Привет! Меня зовут Иван, я работаю DevOps инженером в компании TechCorp.",
		"У нас есть сервер на Ubuntu 22.04.",
		"Мы используем Docker для контейнеризации приложений.",
		"Наш основной стек: PostgreSQL, Redis, Nginx.",
		"Мы развернули мониторинг через Prometheus и Grafana.",
		"У нас есть CI/CD на GitLab CI.",
		"Мы используем Terraform для управления инфраструктурой.",
		"Наши приложения работают в Kubernetes кластере.",
		"Мы используем Ansible для конфигурации серверов.",
		"У нас есть резервное копирование через Bacula.",
		"Мы мониторим логи через ELK Stack.",
		"У нас есть система алертинга через PagerDuty.",
		"Мы используем Vault для управления секретами.",
		"Наш код хранится в GitLab.",
		"Мы используем Jira для управления задачами.",
		"У нас есть документация в Confluence.",
		"Мы проводим код-ревью для всех изменений.",
		"У нас есть автоматизированное тестирование.",
		"Мы используем SonarQube для анализа кода.",
		"У нас есть staging окружение для тестирования.",
		"Как меня зовут?",   // Проверка памяти
		"Где я работаю?",    // Проверка памяти
		"Какой у нас стек?", // Проверка памяти
	}

	for i, userMsg := range testMessages {
		fmt.Printf("\n[Сообщение %d] User: %s\n", i+1, userMsg)

		// Добавляем сообщение пользователя
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userMsg,
		})

		// Оптимизируем контекст перед каждым запросом
		messages = adaptiveContextManagement(ctx, client, messages, maxContextTokens)

		// Показываем статистику
		usedTokens := countTokensInMessages(messages)
		fmt.Printf("📊 Токенов использовано: %d / %d (%.1f%%)\n", usedTokens, maxContextTokens, float64(usedTokens)/float64(maxContextTokens)*100)

		// Отправляем запрос
		resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:       "gpt-4o-mini",
			Messages:    messages,
			Temperature: 0.7,
		})
		if err != nil {
			fmt.Printf("❌ Ошибка: %v\n", err)
			continue
		}

		assistantMsg := resp.Choices[0].Message
		fmt.Printf("Assistant: %s\n", assistantMsg.Content)

		// Добавляем ответ в историю
		messages = append(messages, assistantMsg)
	}

	fmt.Println("\n=== Тест завершен ===")
}

// TODO 1: Реализуйте приблизительный подсчет токенов
// Подсказка: 1 токен ≈ 4 символа для английского, ≈ 3 символа для русского
func estimateTokens(text string) int {
	// TODO: Реализуйте подсчет
	return 0
}

// TODO 2: Реализуйте подсчет токенов во всех сообщениях
// Учтите: System/User/Assistant сообщения, Tool calls тоже занимают токены
func countTokensInMessages(messages []openai.ChatCompletionMessage) int {
	total := 0
	// TODO: Пройдитесь по всем сообщениям и подсчитайте токены
	// Учтите: ToolCalls тоже занимают токены (примерно 80 токенов на вызов)
	return total
}

// TODO 3: Реализуйте обрезку истории
// Всегда сохраняйте System Prompt (messages[0])
// Оставляйте последние сообщения, пока не достигнете maxTokens
func truncateHistory(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	// TODO: Реализуйте обрезку
	// Подсказка: Идите с конца и добавляйте сообщения, пока не достигнете лимита
	return messages
}

// TODO 4: Реализуйте саммаризацию старых сообщений
// Используйте LLM для создания краткого резюме
// Сохраните: важные факты, решения, текущее состояние задачи
func summarizeMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) string {
	// TODO: Соберите текст всех сообщений (кроме System)
	// TODO: Создайте промпт для саммаризации
	// TODO: Вызовите LLM для создания саммари
	// TODO: Верните результат

	// Подсказка: Используйте такой промпт:
	// "Summarize this conversation, keeping only:
	//  1. Important decisions made
	//  2. Key facts discovered
	//  3. Current state of the task
	//  Conversation: [текст]"

	return ""
}

// TODO 5: Реализуйте функцию сжатия контекста через саммаризацию
// Разделите сообщения на "старые" и "новые" (последние 10)
// Сожмите старые через summarizeMessages
// Соберите новый контекст: System + Summary + Recent
func compressOldMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	// TODO: Реализуйте сжатие
	return messages
}

// TODO 6: Реализуйте приоритизацию сообщений
// Сохраните:
// - System Prompt (всегда)
// - Последние 5 сообщений (текущий контекст)
// - Сообщения с результатами инструментов (Role == "tool")
// - Сообщения с ошибками (содержат "error")
func prioritizeMessages(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	// TODO: Реализуйте приоритизацию
	return messages
}

// TODO 7: Реализуйте адаптивное управление контекстом
// Если контекст < 80% — ничего не делаем
// Если 80-90% — применяем приоритизацию
// Если > 90% — применяем саммаризацию
func adaptiveContextManagement(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	usedTokens := countTokensInMessages(messages)
	_ = usedTokens
	// TODO: Реализуйте логику выбора техники оптимизации
	// Подсказка: Используйте threshold80 и threshold90

	return messages
}
