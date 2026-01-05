package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// Step represents a single step in the plan
type Step struct {
	ID           string   // Unique step ID
	Description  string   // Step description
	Dependencies []string // IDs of steps this step depends on
	Status       string   // pending, running, completed, failed
	Result       string   // Execution result
}

// Plan represents the complete task execution plan
type Plan struct {
	ID    string
	Task  string
	Steps []*Step
}

// StepExecutor interface for executing steps
type StepExecutor interface {
	Execute(step *Step) (string, error)
}

// TODO 1: Implement plan creation function via LLM
// Use LLM to decompose task into steps
// Define dependencies between steps
func createPlan(ctx context.Context, client *openai.Client, task string) (*Plan, error) {
	// TODO: Create prompt for task decomposition
	// TODO: Call LLM to get plan
	// TODO: Parse LLM response into Plan structure
	// TODO: Return plan
	
	return nil, fmt.Errorf("not implemented")
}

// TODO 2: Implement function to find ready steps
// Return steps whose all dependencies are completed
// Detect cyclic dependencies
func findReadySteps(plan *Plan) ([]*Step, error) {
	// TODO: Iterate through all steps
	// TODO: Check if all dependencies are completed
	// TODO: Detect cyclic dependencies
	// TODO: Return ready steps
	
	return nil, fmt.Errorf("not implemented")
}

// TODO 3: Implement plan execution with retries
// Execute steps considering dependencies
// Retry failed steps up to maxRetries
func executePlanWithRetries(ctx context.Context, plan *Plan, executor StepExecutor, maxRetries int) error {
	// TODO: Find ready steps
	// TODO: Execute steps
	// TODO: Handle errors (retry, skip, abort)
	// TODO: Track step status
	
	return fmt.Errorf("not implemented")
}

// TODO 4: Implement plan state persistence
// Save plan to file (JSON format)
func savePlanState(planID string, plan *Plan) error {
	// TODO: Serialize plan to JSON
	// TODO: Save to file
	
	return fmt.Errorf("not implemented")
}

// TODO 5: Implement plan state loading
// Load plan from file
func loadPlanState(planID string) (*Plan, error) {
	// TODO: Read file
	// TODO: Deserialize JSON to Plan
	// TODO: Return plan
	
	return nil, fmt.Errorf("not implemented")
}

// Mock executor for testing
type MockExecutor struct{}

func (e *MockExecutor) Execute(step *Step) (string, error) {
	fmt.Printf("Executing step: %s\n", step.Description)
	// Simulate execution
	return fmt.Sprintf("Step %s completed", step.ID), nil
}

func main() {
	// Client setup
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

	// Test task
	task := "Deploy new version of service"

	fmt.Println("=== Lab 10: Planning and Workflows ===")
	fmt.Printf("Task: %s\n\n", task)

	// TODO: Create plan
	plan, err := createPlan(ctx, client, task)
	if err != nil {
		fmt.Printf("Error creating plan: %v\n", err)
		return
	}

	fmt.Printf("Plan created with %d steps\n", len(plan.Steps))

	// TODO: Execute plan
	executor := &MockExecutor{}
	err = executePlanWithRetries(ctx, plan, executor, 3)
	if err != nil {
		fmt.Printf("Error executing plan: %v\n", err)
		return
	}

	fmt.Println("\nPlan executed successfully!")
	
	_ = json.RawMessage{}
}
