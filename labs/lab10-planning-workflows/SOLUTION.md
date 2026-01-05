# Lab 10 Solution: Planning and Workflows

## 📝 Solution Breakdown

### Key Points

1. **Plan creation via LLM:** Use structured prompt to get plan in JSON format.

2. **Dependency resolution:** Always check dependency status before executing step.

3. **Cycle detection:** Check dependency graph for cycles.

4. **State persistence:** Save plan after each completed step.

### 🔍 Complete Solution

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
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

func createPlan(ctx context.Context, client *openai.Client, task string) (*Plan, error) {
	prompt := fmt.Sprintf(`Break down the task into steps with dependencies.
Task: %s

Return plan in JSON format:
{
  "steps": [
    {"id": "step1", "description": "...", "dependencies": []},
    {"id": "step2", "description": "...", "dependencies": ["step1"]}
  ]
}

JSON only, no additional text.`, task)

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}

	// Parse JSON response
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
			// Check if all steps are completed
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
		
		// Execute ready steps
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
			
			// Save state after each step
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

	task := "Deploy new version of service"

	plan, err := createPlan(ctx, client, task)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Plan created with %d steps\n", len(plan.Steps))

	executor := &MockExecutor{}
	if err := executePlanWithRetries(ctx, plan, executor, 3); err != nil {
		panic(err)
	}

	fmt.Println("Plan executed successfully!")
}
```
