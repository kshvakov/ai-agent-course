# Study Guide: Lab 02 — Function Calling (Tools)

## Why This Lab?

A regular LLM returns text. But to create an agent, we need the model to be able to call functions (tools). This turns an LLM from a "talker" into a "worker".

### Real-World Case Study

**Situation:** You've created a chatbot for DevOps. User writes:
- "Check server status web-01"
- Bot responds: "I will check server status web-01 for you..." (text)

**Problem:** Bot can't actually check the server. It only talks.

**Solution:** Function Calling allows the model to call real Go functions.

## Theory in Simple Terms

### How Does Function Calling Work?

1. **You describe a function** in JSON Schema format
2. **LLM sees the description** and decides: "I need to call this function"
3. **LLM generates JSON** with function name and arguments
4. **Your code parses JSON** and executes the real function
5. **Result is returned** to LLM for further processing

### Why Don't All Models Support Tools?

Function Calling is the result of special training. If the model hasn't seen function call examples, it will simply continue the dialogue with text.

**How to check:** Run Lab 00 before this lab!

## Execution Algorithm

### Step 1: Tool Definition

```go
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "get_server_status",
            Description: "Get the status of a server by IP address",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "ip": {
                        "type": "string",
                        "description": "IP address of the server"
                    }
                },
                "required": ["ip"]
            }`),
        },
    },
}
```

**Important:** `Description` is the most important field! LLM relies on it.

### Step 2: Sending Request with Tools

```go
req := openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,
    Tools:    tools,  // Pass list of tools
}
```

### Step 3: Response Processing

```go
resp, err := client.CreateChatCompletion(ctx, req)
msg := resp.Choices[0].Message

// Check if model wants to call a function
if len(msg.ToolCalls) > 0 {
    // Model wants to call a tool!
    call := msg.ToolCalls[0]
    fmt.Printf("Function: %s\n", call.Function.Name)
    fmt.Printf("Arguments: %s\n", call.Function.Arguments)
    
    // Parse arguments
    var args struct {
        IP string `json:"ip"`
    }
    json.Unmarshal([]byte(call.Function.Arguments), &args)
    
    // Call real function
    result := runGetServerStatus(args.IP)
    fmt.Printf("Result: %s\n", result)
} else {
    // Model responded with text
    fmt.Println("Text response:", msg.Content)
}
```

## Common Mistakes

### Mistake 1: Model Doesn't Call Function

**Symptom:** `len(msg.ToolCalls) == 0`, model responds with text.

**Causes:**
1. Model not trained on Function Calling
2. Poor tool description (`Description` unclear)
3. Temperature > 0 (too random)

**Solution:**
1. Check model via Lab 00
2. Improve `Description`: make it specific and clear
3. Set `Temperature = 0`

### Mistake 2: Broken JSON in Arguments

**Symptom:** `json.Unmarshal` returns error.

**Example:**
```json
{"ip": "192.168.1.10"  // Missing closing brace
```

**Solution:**
```go
// Validate JSON before parsing
if !json.Valid([]byte(call.Function.Arguments)) {
    return "Error: Invalid JSON", nil
}
```

### Mistake 3: Wrong Function Name

**Symptom:** Model calls function with different name.

**Example:**
```json
{"name": "check_server"}  // But function is called "get_server_status"
```

**Solution:**
```go
// Validate function name
allowedFunctions := map[string]bool{
    "get_server_status": true,
}
if !allowedFunctions[call.Function.Name] {
    return "Error: Unknown function", nil
}
```

## Mini-Exercises

### Exercise 1: Add Second Tool

Create a `ping_host(host string)` tool and verify that the model correctly chooses between two tools.

### Exercise 2: Improve Description

Try different descriptions and see how it affects model choice:

```go
// Option 1: Short
Description: "Ping a host"

// Option 2: Detailed
Description: "Ping a host to check network connectivity. Returns latency in milliseconds."
```

## Completion Criteria

✅ **Completed:**
- Model successfully calls function
- Arguments parsed correctly
- Function result processed

❌ **Not completed:**
- Model doesn't call function (only text)
- JSON arguments broken
- Code doesn't compile

---

**Next step:** After successfully completing Lab 02, proceed to [Lab 03: Architecture](../lab03-real-world/README.md)
