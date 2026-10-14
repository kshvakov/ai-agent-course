# Решение: Lab 10 — Planning и Workflows

## 📝 Разбор решения

### Ключевые моменты

1. **Создание плана через LLM:** Используйте структурированный промпт для получения плана в JSON формате.

2. **Разрешение зависимостей:** Всегда проверяйте статус зависимостей перед выполнением шага.

3. **Обнаружение циклов:** Проверяйте граф зависимостей на наличие циклов.

4. **Сохранение состояния:** Сохраняйте план после каждого выполненного шага.

### 🔍 Полное решение

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kshvakov/agent/llm"
)

type Step struct {
	ID           string
	Description  string
	Dependencies []string
	Status       string
	Result       string
}

type Plan struct {
	ID    string
	Task  string
	Steps []*Step
}

type StepExecutor interface {
	Execute(step *Step) (string, error)
}

func createPlan(ctx context.Context, client llm.Client, model, task string) (*Plan, error) {
	prompt := fmt.Sprintf(`Разбей задачу на шаги с зависимостями.
Задача: %s

Верни план в формате JSON:
{
  "steps": [
    {"id": "step1", "description": "...", "dependencies": []},
    {"id": "step2", "description": "...", "dependencies": ["step1"]}
  ]
}

Только JSON, без дополнительного текста.`, task)

	resp, err := client.Complete(ctx, llm.Request{
		Model: model, // Из LLM_MODEL (по умолчанию glm-5.3)
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}

	// Парсим JSON ответ
	var planData struct {
		Steps []struct {
			ID           string   `json:"id"`
			Description  string   `json:"description"`
			Dependencies []string `json:"dependencies"`
		} `json:"steps"`
	}

	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &planData); err != nil {
		return nil, err
	}

	plan := &Plan{
		ID:    fmt.Sprintf("plan_%d", os.Getpid()),
		Task:  task,
		Steps: make([]*Step, len(planData.Steps)),
	}

	for i, s := range planData.Steps {
		plan.Steps[i] = &Step{
			ID:           s.ID,
			Description:  s.Description,
			Dependencies: s.Dependencies,
			Status:       "pending",
		}
	}

	return plan, nil
}

func findStep(plan *Plan, id string) *Step {
	for _, step := range plan.Steps {
		if step.ID == id {
			return step
		}
	}
	return nil
}

func findReadySteps(plan *Plan) ([]*Step, error) {
	var ready []*Step
	
	for _, step := range plan.Steps {
		if step.Status != "pending" {
			continue
		}
		
		allDepsCompleted := true
		for _, depID := range step.Dependencies {
			dep := findStep(plan, depID)
			if dep == nil {
				return nil, fmt.Errorf("dependency %s not found", depID)
			}
			if dep.Status != "completed" {
				allDepsCompleted = false
				break
			}
		}
		
		if allDepsCompleted {
			ready = append(ready, step)
		}
	}
	
	return ready, nil
}

func executePlanWithRetries(ctx context.Context, plan *Plan, executor StepExecutor, maxRetries int) error {
	for {
		ready, err := findReadySteps(plan)
		if err != nil {
			return err
		}
		
		if len(ready) == 0 {
			// Проверяем, все ли выполнено
			allCompleted := true
			for _, step := range plan.Steps {
				if step.Status != "completed" {
					allCompleted = false
					break
				}
			}
			if allCompleted {
				return nil
			}
			return fmt.Errorf("deadlock: no ready steps")
		}
		
		// Выполняем готовые шаги
		for _, step := range ready {
			step.Status = "running"
			
			var result string
			var err error
			retries := 0
			
			for retries < maxRetries {
				result, err = executor.Execute(step)
				if err == nil {
					break
				}
				retries++
			}
			
			if err != nil {
				step.Status = "failed"
				return fmt.Errorf("step %s failed after %d retries: %v", step.ID, maxRetries, err)
			}
			
			step.Status = "completed"
			step.Result = result
			
			// Сохраняем состояние после каждого шага
			savePlanState(plan.ID, plan)
		}
	}
}

func savePlanState(planID string, plan *Plan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("plan_%s.json", planID), data, 0644)
}

func loadPlanState(planID string) (*Plan, error) {
	data, err := os.ReadFile(fmt.Sprintf("plan_%s.json", planID))
	if err != nil {
		return nil, err
	}
	
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}
	
	return &plan, nil
}

type MockExecutor struct{}

func (e *MockExecutor) Execute(step *Step) (string, error) {
	fmt.Printf("Executing: %s\n", step.Description)
	return fmt.Sprintf("Step %s completed", step.ID), nil
}

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

	ctx := context.Background()

	task := "Deploy new version of service"

	plan, err := createPlan(ctx, client, cfg.Model, task)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("Plan created with %d steps\n", len(plan.Steps))

	executor := &MockExecutor{}
	if err := executePlanWithRetries(ctx, plan, executor, 3); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Plan executed successfully!")
}
```

