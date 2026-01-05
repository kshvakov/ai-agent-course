# Model and Decoding

## Why This Chapter?

Agent works, but sometimes returns "broken" JSON in tool calls or behaves unpredictably. Without proper model selection and decoding configuration, you cannot guarantee quality and determinism.

### Real-World Case Study

**Situation:** Agent uses local model for tool calling. Sometimes model returns invalid JSON, and agent crashes.

**Problem:** Model not suitable for tool calling, no determinism (Temperature > 0), no JSON mode.

**Solution:** Capability benchmark before development, determinism (Temperature = 0), JSON mode for structured outputs, model selection for task.

## Theory in Simple Terms

### What Is Capability Benchmark?

Capability Benchmark is a test suite for checking model before development. Checks: JSON generation, Instruction following, Function calling.

### What Is Determinism?

Determinism is output predictability. At Temperature = 0, model always returns same result for same input.

### What Is JSON Mode?

JSON mode is a mode where model guaranteed returns valid JSON. This reduces probability of "broken" JSON in tool calls.

## How It Works (Step-by-Step)

### Step 1: Capability Benchmark

Check model before development (see `labs/lab00-capability-check/main.go`):

```go
func runCapabilityBenchmark(model string) (bool, error) {
    // Check JSON generation
    jsonTest := testJSONGeneration(model)
    
    // Check Instruction following
    instructionTest := testInstructionFollowing(model)
    
    // Check Function calling
    functionTest := testFunctionCalling(model)
    
    return jsonTest && instructionTest && functionTest, nil
}
```

### Step 2: Determinism

Use Temperature = 0 for tool calling:

```go
func createToolCallRequest(messages []openai.ChatCompletionMessage, tools []openai.Tool) openai.ChatCompletionRequest {
    return openai.ChatCompletionRequest{
        Model:       "gpt-4",
        Messages:    messages,
        Tools:       tools,
        Temperature: 0, // Determinism for tool calling
    }
}
```

### Step 3: JSON Mode

Use JSON mode for structured outputs:

```go
func createStructuredRequest(messages []openai.ChatCompletionMessage) openai.ChatCompletionRequest {
    return openai.ChatCompletionRequest{
        Model: "gpt-4-turbo",
        Messages: messages,
        ResponseFormat: &openai.ChatCompletionResponseFormat{
            Type: openai.ChatCompletionResponseFormatTypeJSONObject,
        },
        Temperature: 0,
    }
}
```

### Step 4: Model Selection for Task

Use cheaper models for simple tasks:

```go
func selectModel(taskComplexity string) string {
    switch taskComplexity {
    case "simple":
        return openai.GPT3Dot5Turbo // Cheaper and faster
    case "complex":
        return openai.GPT4 // Better quality, but more expensive
    default:
        return openai.GPT3Dot5Turbo
    }
}
```

## Where to Integrate in Our Code

### Integration Point 1: Capability Check

In `labs/lab00-capability-check/main.go`, benchmark already exists. Use it before development.

### Integration Point 2: Tool Calling

In `labs/lab02-tools/main.go`, set Temperature = 0:

```go
req := openai.ChatCompletionRequest{
    Model:       openai.GPT3Dot5Turbo,
    Messages:    messages,
    Tools:       tools,
    Temperature: 0, // Determinism
}
```

### Integration Point 3: SOP and Determinism

In `labs/lab06-incident/SOLUTION.md`, Temperature = 0 is already used for determinism.

## Common Mistakes

### Mistake 1: Model Not Checked Before Development

**Symptom:** Model not suitable for tool calling, returns invalid JSON.

**Solution:** Run capability benchmark before development.

### Mistake 2: Temperature > 0 for Tool Calling

**Symptom:** Agent behaves unpredictably, same requests give different results.

**Solution:** Use Temperature = 0 for tool calling.

### Mistake 3: No JSON Mode

**Symptom:** Model returns "broken" JSON in tool calls.

**Solution:** Use JSON mode if model supports it.

## Completion Criteria / Checklist

✅ **Completed:**
- Model checked via capability benchmark
- Temperature = 0 for tool calling
- JSON mode used for structured outputs
- Model selected for task

❌ **Not completed:**
- Model not checked
- Temperature > 0 for tool calling
- No JSON mode

## Connection with Other Chapters

- **Capability Benchmark:** Model checking — [Lab 00: Capability Check](../../labs/lab00-capability-check/METHOD.md)
- **LLM Physics:** Fundamental concepts — [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md)
- **Function Calling:** Tool calling — [Chapter 04: Tools and Function Calling](../04-tools-and-function-calling/README.md)
- **Cost Engineering:** Model selection for cost optimization — [Cost & Latency Engineering](cost_latency.md)

---

**Navigation:** [← Multi-Agent in Production](multi_agent_in_prod.md) | [Chapter 12 Table of Contents](README.md)
