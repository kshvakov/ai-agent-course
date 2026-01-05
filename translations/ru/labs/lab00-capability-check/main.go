package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type TestResult struct {
	Name    string
	Passed  bool
	Details string
}

func main() {
	token := os.Getenv("OPENAI_API_KEY")
	if token == "" { token = "dummy" }
	config := openai.DefaultConfig(token)
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" { config.BaseURL = baseURL }
	client := openai.NewClientWithConfig(config)
	ctx := context.Background()

	fmt.Println("🔬 Starting Model Capability Analysis...")
	fmt.Printf("Endpoint: %s\n", config.BaseURL)

	results := []TestResult{}

	// TEST 1: Basic Sanity
	results = append(results, runTest(ctx, client, "1. Basic Sanity", 
		"Say exactly 'Hello World'", 
		func(response string) bool { return strings.Contains(strings.ToLower(response), "hello world") },
	))

	// TEST 2: Instruction Following (Constraints)
	results = append(results, runTest(ctx, client, "2. Instruction Following", 
		"Reply with the word 'Apple' and nothing else. No punctuation.", 
		func(response string) bool { return strings.TrimSpace(response) == "Apple" },
	))

	// TEST 3: JSON Generation
	results = append(results, runTest(ctx, client, "3. JSON Generation", 
		"Generate a JSON object with field 'status' set to 'ok'. Do not use markdown blocks.", 
		func(response string) bool {
			var js map[string]interface{}
			// Try to find JSON if wrapped in markdown
			clean := strings.Trim(response, "`json \n")
			return json.Unmarshal([]byte(clean), &js) == nil && js["status"] == "ok"
		},
	))

	// TEST 4: Function Calling
	results = append(results, runToolTest(ctx, client))

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

func runTest(ctx context.Context, client *openai.Client, name, prompt string, validator func(string) bool) TestResult {
	fmt.Printf("Running %s...\n", name)
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: prompt}},
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

func runToolTest(ctx context.Context, client *openai.Client) TestResult {
	fmt.Println("Running 4. Function Calling...")
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name: "test_tool",
				Description: "Call this tool to pass the test",
				Parameters: json.RawMessage(`{"type": "object", "properties": {"foo": {"type": "string"}}}`),
			},
		},
	}

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Call the test_tool please."}},
		Tools: tools,
	})

	if err != nil {
		return TestResult{"4. Function Calling", false, fmt.Sprintf("API Error: %v", err)}
	}

	if len(resp.Choices[0].Message.ToolCalls) > 0 {
		return TestResult{"4. Function Calling", true, "Model successfully generated a tool call."}
	}

	return TestResult{"4. Function Calling", false, fmt.Sprintf("Model responded with text instead of tool: '%s'", resp.Choices[0].Message.Content)}
}

