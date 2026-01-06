# Решение: Lab 13 — Tool Retrieval & Pipeline Building

## Полная реализация

Вот полное решение со всеми реализованными TODO:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// ... (toolCatalog, sampleLogs, типы остаются теми же) ...

// Реализация searchToolCatalog
func searchToolCatalog(query string, topK int) []ToolDefinition {
	var results []ToolDefinition
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	// Оценка каждого инструмента по релевантности
	type scoredTool struct {
		tool  ToolDefinition
		score int
	}
	var scored []scoredTool

	for _, tool := range toolCatalog {
		score := 0
		toolDescLower := strings.ToLower(tool.Description)

		// Подсчет совпадений в описании
		for _, word := range queryWords {
			if strings.Contains(toolDescLower, word) {
				score += 2 // Совпадения в описании важнее
			}
		}

		// Подсчет совпадений в тегах
		for _, tag := range tool.Tags {
			tagLower := strings.ToLower(tag)
			for _, word := range queryWords {
				if strings.Contains(tagLower, word) {
					score += 1
				}
			}
		}

		if score > 0 {
			scored = append(scored, scoredTool{tool: tool, score: score})
		}
	}

	// Сортировка по оценке (по убыванию)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Возврат top-k
	if len(scored) > topK {
		scored = scored[:topK]
	}

	results = make([]ToolDefinition, len(scored))
	for i, s := range scored {
		results[i] = s.tool
	}

	return results
}

// Реализации выполнения инструментов
func executeGrep(input string, pattern string) string {
	lines := strings.Split(input, "\n")
	var result []string
	for _, line := range lines {
		if strings.Contains(line, pattern) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func executeSort(input string) string {
	lines := strings.Split(input, "\n")
	// Удаляем пустые строки
	var nonEmpty []string
	for _, line := range lines {
		if line != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	sort.Strings(nonEmpty)
	return strings.Join(nonEmpty, "\n")
}

func executeHead(input string, lines int) string {
	allLines := strings.Split(input, "\n")
	if lines > len(allLines) {
		lines = len(allLines)
	}
	return strings.Join(allLines[:lines], "\n")
}

func executeUniq(input string, count bool) string {
	lines := strings.Split(input, "\n")
	// Удаляем пустые строки
	var nonEmpty []string
	for _, line := range lines {
		if line != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}

	if count {
		// Подсчет вхождений (как uniq -c)
		counts := make(map[string]int)
		for _, line := range nonEmpty {
			counts[line]++
		}

		var result []string
		for line, cnt := range counts {
			result = append(result, fmt.Sprintf("%d %s", cnt, line))
		}
		// Сортировка по количеству (по убыванию)
		sort.Slice(result, func(i, j int) bool {
			return result[i] > result[j]
		})
		return strings.Join(result, "\n")
	} else {
		// Простая дедупликация
		seen := make(map[string]bool)
		var result []string
		for _, line := range nonEmpty {
			if !seen[line] {
				seen[line] = true
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n")
	}
}

// executeToolStep остается тем же
func executeToolStep(toolName string, args map[string]interface{}, input string) (string, error) {
	switch toolName {
	case "grep":
		pattern, ok := args["pattern"].(string)
		if !ok {
			return "", fmt.Errorf("grep requires 'pattern' argument")
		}
		return executeGrep(input, pattern), nil
	case "sort":
		return executeSort(input), nil
	case "head":
		lines, ok := args["lines"].(float64)
		if !ok {
			return "", fmt.Errorf("head requires 'lines' argument")
		}
		return executeHead(input, int(lines)), nil
	case "uniq":
		count := false
		if c, ok := args["count"].(bool); ok {
			count = c
		}
		return executeUniq(input, count), nil
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Реализация executePipeline
func executePipeline(pipelineJSON string, inputData string) (string, error) {
	// Валидация JSON
	if !json.Valid([]byte(pipelineJSON)) {
		return "", fmt.Errorf("invalid JSON")
	}

	// Парсинг JSON
	var pipeline Pipeline
	if err := json.Unmarshal([]byte(pipelineJSON), &pipeline); err != nil {
		return "", fmt.Errorf("failed to parse pipeline: %v", err)
	}

	// Валидация уровня риска
	if pipeline.RiskLevel == "dangerous" {
		return "", fmt.Errorf("dangerous pipeline requires human approval")
	}

	// Валидация шагов
	if len(pipeline.Steps) == 0 {
		return "", fmt.Errorf("pipeline has no steps")
	}

	// Выполнение шагов последовательно
	currentData := inputData
	for i, step := range pipeline.Steps {
		result, err := executeToolStep(step.Tool, step.Args, currentData)
		if err != nil {
			return "", fmt.Errorf("step %d (%s) failed: %v", i, step.Tool, err)
		}
		currentData = result
	}

	return currentData, nil
}

// main остается тем же
```

## Ключевые моменты

1. **Поиск инструментов:** Оценивает инструменты по совпадению слов запроса в описании (вес 2) и тегах (вес 1), затем возвращает top-k, отсортированные по оценке.

2. **Выполнение инструментов:** Каждый инструмент работает со строковыми данными (симулируя Linux-команды):
   - `grep`: Фильтрует строки, содержащие паттерн
   - `sort`: Сортирует строки по алфавиту
   - `head`: Возвращает первые N строк
   - `uniq`: Дедуплицирует (с опциональным подсчетом)

3. **Выполнение пайплайна:** 
   - Валидирует JSON и обязательные поля
   - Отклоняет опасные пайплайны
   - Выполняет шаги последовательно (каждый вывод становится следующим входом)

4. **Поток агента:**
   - Агент вызывает `search_tool_catalog("error filter sort")`
   - Получает релевантные инструменты: `[grep, sort, uniq, head]`
   - Строит pipeline JSON
   - Вызывает `execute_pipeline(pipeline_json, log_data)`
   - Возвращает результат

## Ожидаемый результат

Для запроса "Найди топ-5 самых частых строк с ошибками":
- Пайплайн: `grep("ERROR") → uniq(count=true) → sort() → head(5)`
- Результат: Топ-5 строк с ошибками, отсортированных по частоте

