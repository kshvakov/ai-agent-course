package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kshvakov/agent/llm"
)

type TestResult struct {
	Name    string
	Passed  bool
	Details string
}

func main() {
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

	fmt.Println("🔬 Starting Model Capability Analysis...")
	fmt.Printf("Endpoint: %s\n", cfg.BaseURL)
	fmt.Printf("Model: %s\n", cfg.Model)

	results := []TestResult{}

	// TEST 1: Basic Sanity
	results = append(results, runTest(ctx, client, cfg.Model, "1. Basic Sanity",
		"Say exactly 'Hello World'",
		func(response string) bool { return strings.Contains(strings.ToLower(response), "hello world") },
	))

	// TEST 2: Instruction Following (Constraints)
	results = append(results, runTest(ctx, client, cfg.Model, "2. Instruction Following",
		"Reply with the word 'Apple' and nothing else. No punctuation.",
		func(response string) bool { return strings.TrimSpace(response) == "Apple" },
	))

	// TEST 3: JSON Generation
	results = append(results, runTest(ctx, client, cfg.Model, "3. JSON Generation",
		"Generate a JSON object with field 'status' set to 'ok'. Do not use markdown blocks.",
		func(response string) bool {
			var js map[string]any
			// Try to find JSON if wrapped in markdown
			clean := strings.Trim(response, "`json \n")
			return json.Unmarshal([]byte(clean), &js) == nil && js["status"] == "ok"
		},
	))

	// TEST 4: Function Calling
	results = append(results, runToolTest(ctx, client, cfg.Model))

	// REPORT
	fmt.Println("\n📋 FINAL REPORT:")
	allPassed := true
	for _, r := range results {
		icon := "✅"
		if !r.Passed {
			icon = "❌"
			allPassed = false
		}
		fmt.Printf("%s %s\n   Details: %s\n", icon, r.Name, r.Details)
	}

	if allPassed {
		fmt.Println("\n🎉 EXCELLENT! This model is ready for the course.")
	} else {
		fmt.Println("\n⚠️ WARNING! This model has limitations. Some labs might fail.")
	}
}

func runTest(ctx context.Context, client llm.Client, model, name, prompt string, validator func(string) bool) TestResult {
	fmt.Printf("Running %s...\n", name)
	resp, err := client.Complete(ctx, llm.Request{
		Model:       model,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0,
	})

	if err != nil {
		return TestResult{name, false, fmt.Sprintf("API Error: %v", err)}
	}

	content := resp.Choices[0].Message.Content
	passed := validator(content)
	details := fmt.Sprintf("Input: '%s' | Output: '%s'", prompt, content)

	return TestResult{name, passed, details}
}

func runToolTest(ctx context.Context, client llm.Client, model string) TestResult {
	fmt.Println("Running 4. Function Calling...")
	tools := []llm.ToolDef{
		{
			Name:        "get_time",
			Description: "Get the current time",
			Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		},
	}

	resp, err := client.Complete(ctx, llm.Request{
		Model:    model,
		Messages: []llm.Message{{Role: "user", Content: "Call the get_time tool please."}},
		Tools:    tools,
	})

	if err != nil {
		return TestResult{"4. Function Calling", false, fmt.Sprintf("API Error: %v", err)}
	}

	if len(resp.Choices[0].Message.ToolCalls) > 0 {
		return TestResult{"4. Function Calling", true, "Model successfully generated a tool call."}
	}

	return TestResult{"4. Function Calling", false, fmt.Sprintf("Model responded with text instead of tool: '%s'", resp.Choices[0].Message.Content)}
}
