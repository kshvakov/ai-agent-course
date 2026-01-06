# Manual: Lab 00 — Model Capability Benchmark

## Why Is This Needed?

Before building complex agents, we must **scientifically confirm** that our model (especially local) has the necessary capabilities. In engineering, this is called **Characterization**.

We don't trust labels ("Super-Pro-Max Model"). We trust tests.

### Real-World Case Study

**Situation:** You downloaded the "Llama-3-8B-Instruct" model and started building an agent. After an hour of work, you discovered that the model doesn't call tools, only writes text.

**Problem:** You spent time debugging code, though the problem was in the model.

**Solution:** Run Lab 00 **before** starting work. This saves hours.

## Theory in Simple Terms

### What Do We Check?

1. **Basic Sanity (Basic Functionality)**
   - Model responds to requests
   - No critical API errors

2. **Instruction Following**
   - Model can strictly adhere to constraints
   - Important for agents: they must return strictly defined formats

3. **JSON Generation**
   - Model can generate valid syntax
   - All tool interaction is built on JSON

4. **Function Calling (Tool Usage)**
   - Specific model skill to recognize function definitions
   - Without this, Lab 02 and beyond are impossible

### Why Don't All Models Know Tools?

LLM (Large Language Model) is a probabilistic text generator. It doesn't "know" about functions.

The **Function Calling** mechanism is a result of special training (Fine-Tuning). Model developers add thousands of examples to the training set:

```
User: "Check weather"
Assistant: <special_token>call_tool{"name": "weather"}<end_token>
```

If you downloaded a "bare" Llama 3 (Base model), it hasn't seen these examples. It will simply continue the dialogue with text.

## Execution Algorithm

### Step 1: Running Tests

```bash
cd labs/lab00-capability-check
export OPENAI_BASE_URL="http://localhost:1234/v1"
export OPENAI_API_KEY="lm-studio"
go run main.go
```

### Step 2: Analyzing Results

Tests will output a report:

```
✅ 1. Basic Sanity - PASSED
✅ 2. Instruction Following - PASSED
✅ 3. JSON Generation - PASSED
❌ 4. Function Calling - FAILED
```

### Step 3: Interpretation

- **If all tests passed:** Model is ready for the course. You can continue.
- **If Function Calling failed:** Model is not suitable for Lab 02-08. You need a different model.

## Common Errors

### Error 1: "API Error: connection refused"

**Cause:** Local server (LM Studio/Ollama) is not running.

**Solution:**
1. Start LM Studio
2. Click "Start Server" (usually port 1234)
3. Check that `OPENAI_BASE_URL` points to the correct port

### Error 2: "Function Calling - FAILED"

**Cause:** Model is not trained on Function Calling.

**Solution:**
1. Download a model with tools support:
   - `Hermes-2-Pro-Llama-3-8B`
   - `Mistral-7B-Instruct-v0.2`
   - `Llama-3-8B-Instruct` (some versions)
2. Restart tests

### Error 3: "JSON Generation - FAILED"

**Cause:** Model generates broken JSON (missing brackets, quotes).

**Solution:**
1. Try a different model
2. Or use `Temperature = 0` (but this doesn't always help)

## Mini-Exercises

### Exercise 1: Add Your Own Test

Add a test to check "model must not use forbidden words":

```go
runTest(ctx, client, "5. Safety Check",
    "Say 'Hello' but do NOT use the word 'hi'",
    func(response string) bool {
        return !strings.Contains(strings.ToLower(response), "hi")
    },
)
```

### Exercise 2: Measure Latency

Add response time measurement:

```go
start := time.Now()
resp, err := client.CreateChatCompletion(...)
latency := time.Since(start)
fmt.Printf("Latency: %v\n", latency)
```

## Completion Criteria

✅ **Completed:** All 4 tests passed successfully  
⚠️ **Partial:** 3 out of 4 tests passed (can continue, but with caution)  
❌ **Not completed:** Function Calling failed (need a different model)

---

**Next step:** After successfully completing Lab 00, proceed to [Lab 01: Basics](../lab01-basics/README.md)
