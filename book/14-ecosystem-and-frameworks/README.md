# 14. Ecosystem and Frameworks

## Why This Chapter?

When creating AI agents, you face a choice: write everything from scratch or use a framework? Both approaches have pros and cons, and understanding when to choose which is critical for long-term success.

This chapter helps you make informed architecture decisions, avoid vendor lock-in, and use existing solutions when they fit.

### Real-World Case Study

**Situation:** You need to create a DevOps agent. You can:
- Use a popular framework that provides everything out of the box
- Build your own runtime tailored to your needs

**Problem:**
- Framework approach: Fast start, but you're locked into their abstractions. When custom logic is needed, you fight the framework.
- Custom approach: Full control, but you reinvent the wheel. Every function (tool execution, memory, planning) requires implementation.

**Solution:** Understand tradeoffs. Choose framework when speed matters and requirements are standard. Choose custom when specific control is needed or there are unique constraints.

## Theory in Simple Terms

### What Are Agent Frameworks?

Agent frameworks are libraries or platforms that provide the following:
- **Tool execution infrastructure** — handling function calling, validation, error handling
- **Memory management** — context windows, summarization, state persistence
- **Planning patterns** — ReAct loops, workflow orchestration, task decomposition
- **Multi-agent coordination** — supervisor patterns, context isolation, routing

**Takeaway:** Frameworks abstract common patterns, but they also impose constraints. Understanding these constraints helps you decide when to use them and when to build custom solutions.

### Custom Runtime vs Framework

**Custom Runtime:**
- [x] Full control over every component
- [x] No vendor lock-in
- [x] Optimized for your specific use case
- [ ] More code to write and maintain
- [ ] Need to implement common patterns yourself

**Framework:**
- [x] Fast development, proven patterns
- [x] Community support and examples
- [x] Handles edge cases you might miss
- [ ] Less flexibility, harder to customize
- [ ] Potential vendor lock-in
- [ ] May include features you don't need

## How to Choose?

### Decision Criteria

**Choose Custom Runtime when:**
1. **Unique requirements** — Your use case doesn't fit standard patterns
2. **Performance critical** — Need fine control over latency/cost
3. **Minimal dependencies** — Want to avoid external dependencies
4. **Learning goal** — Want to deeply understand internals
5. **Long-term control** — Need to independently maintain and evolve system

**Choose Framework when:**
1. **Standard use case** — Your requirements match common patterns
2. **Time to market** — Need to launch quickly
3. **Team familiarity** — Your team already knows the framework
4. **Rapid prototyping** — Exploring ideas and need quick iterations
5. **Community support** — Benefit from examples and community knowledge

### Portability Considerations

**Avoid vendor lock-in through:**
- **Interface abstraction** — Define your interfaces for tools, memory, planning
- **Minimal framework coupling** — Use framework for orchestration, but keep business logic separate
- **Standard protocols** — Prefer standard formats (JSON Schema for tools, OpenTelemetry for observability)
- **Gradual migration path** — Design so components can be changed later

### Working with JSON Schema in Go

When using JSON Schema for tool definitions, prefer Go packages for validation and generation instead of raw `json.RawMessage`. This ensures type safety and better error handling.

**Example: Using `github.com/xeipuuv/gojsonschema` for validation:**

```go
import (
    "github.com/xeipuuv/gojsonschema"
)

// Define tool schema as JSON Schema
const pingToolSchema = `{
  "type": "object",
  "properties": {
    "host": {
      "type": "string",
      "description": "Hostname or IP address to ping"
    },
    "count": {
      "type": "integer",
      "description": "Number of ping packets",
      "default": 4,
      "minimum": 1,
      "maximum": 10
    }
  },
  "required": ["host"]
}`

// Validate tool arguments against schema
func validateToolArgs(schemaJSON string, args map[string]any) error {
    schemaLoader := gojsonschema.NewStringLoader(schemaJSON)
    documentLoader := gojsonschema.NewGoLoader(args)
    
    result, err := gojsonschema.Validate(schemaLoader, documentLoader)
    if err != nil {
        return fmt.Errorf("schema validation error: %w", err)
    }
    
    if !result.Valid() {
        errors := make([]string, 0, len(result.Errors()))
        for _, desc := range result.Errors() {
            errors = append(errors, desc.String())
        }
        return fmt.Errorf("validation failed: %s", strings.Join(errors, "; "))
    }
    
    return nil
}

// Usage when executing tool
func executePing(args map[string]any) (string, error) {
    // Validate arguments before execution
    if err := validateToolArgs(pingToolSchema, args); err != nil {
        return "", err
    }
    
    host := args["host"].(string)
    count := 4
    if c, ok := args["count"].(float64); ok {
        count = int(c)
    }
    
    // Execute ping...
    return fmt.Sprintf("Pinged %s %d times", host, count), nil
}
```

**Example: Using `github.com/invopop/jsonschema` for schema generation:**

```go
import (
    "encoding/json"
    "github.com/invopop/jsonschema"
)

// Define tool parameters as Go struct
type PingParams struct {
    Host  string `json:"host" jsonschema:"required,title=Host,description=Hostname or IP address to ping"`
    Count int    `json:"count" jsonschema:"default=4,minimum=1,maximum=10,title=Count,description=Number of ping packets"`
}

// Generate JSON Schema from struct
func generateToolSchema(params any) (json.RawMessage, error) {
    reflector := jsonschema.Reflector{
        ExpandedStruct: true,
        DoNotReference: false,
    }
    
    schema := reflector.Reflect(params)
    schemaJSON, err := json.Marshal(schema)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal schema: %w", err)
    }
    
    return json.RawMessage(schemaJSON), nil
}

// Usage: Generate schema for tool
func registerPingTool() {
    params := PingParams{}
    schema, err := generateToolSchema(params)
    if err != nil {
        panic(err)
    }
    
    tool := Tool{
        Name:        "ping",
        Description: "Ping a host to check connectivity",
        Schema:      schema, // Use generated schema instead of raw JSON
    }
    
    registry.Register("ping", tool)
}
```

**Advantages of using JSON Schema packages:**
- **Type safety** — Generate schemas from Go structs
- **Validation** — Validate arguments before tool execution
- **Error messages** — Clear validation errors
- **Maintainability** — Single source of truth (Go struct)
- **Documentation** — Automatically generated schema descriptions

## Common Patterns in Frameworks

Most frameworks implement similar patterns:

### Pattern 1: Tool Registry

```go
// Abstract interface (works with any framework or custom)
type ToolRegistry interface {
    Register(name string, tool Tool) error
    Get(name string) (Tool, error)
    List() []string
}

// Framework may provide:
type FrameworkToolRegistry struct {
    tools map[string]Tool
}

func (r *FrameworkToolRegistry) Register(name string, tool Tool) error {
    r.tools[name] = tool
    return nil
}
```

**Takeaway:** Define your interfaces. Framework becomes an implementation detail.

### Pattern 2: Agent Loop

```go
// Abstract agent loop interface
type AgentLoop interface {
    Run(ctx context.Context, input string) (string, error)
    AddTool(tool Tool) error
    SetMemory(memory Memory) error
}

// Your code uses interface, not framework directly
func processRequest(agent AgentLoop, userInput string) (string, error) {
    return agent.Run(context.Background(), userInput)
}
```

**Takeaway:** Dependency injection allows changing implementations.

### Pattern 3: Memory Abstraction

```go
// Abstract memory interface
type Memory interface {
    Store(key string, value any) error
    Retrieve(key string) (any, error)
    Search(query string) ([]any, error)
}

// Framework memory implements your interface
type FrameworkMemory struct {
    // Framework-specific implementation
}

func (m *FrameworkMemory) Store(key string, value any) error {
    // Adapt framework API to your interface
}
```

**Takeaway:** Your interfaces define the contract. Frameworks provide implementations.

## Frameworks and Ecosystem

### Overview of Real Frameworks

In practice, most teams pick one of the popular frameworks. Here are the main players:

**LangGraph (Python, LangChain).** Framework for building agents using state graphs. Each agent step is a graph node; transitions are defined by conditions. Best for complex workflows with branching and cycles.

**CrewAI (Python).** Framework for multi-agent systems. Agents are organized into "crews" with roles and tasks. Convenient when multiple agents collaborate toward a common goal.

**AutoGen (Microsoft, Python).** Framework for multi-agent systems focused on agent-to-agent dialogue. Agents communicate via messages, supports human-in-the-loop.

**Semantic Kernel (Microsoft, .NET/Python).** Orchestrator framework that integrates LLMs with existing code through "plugins". Geared toward enterprise scenarios. Supports .NET and Python.

### Comparison Table

| Framework | Language | Strengths | Weaknesses |
|-----------|----------|-----------|------------|
| **LangGraph** | Python | Graph-based workflows, flexible state, streaming | Complex API, steep learning curve |
| **CrewAI** | Python | Simple multi-agent coordination, roles | Less flexible for non-standard patterns |
| **AutoGen** | Python | Conversational multi-agent systems, Microsoft backing | Heavyweight, hard to debug |
| **Semantic Kernel** | .NET, Python | Enterprise-ready, Azure integration | Tied to Microsoft ecosystem |

### MCP Ecosystem

MCP (Model Context Protocol) is an open protocol for connecting tools to LLMs. The MCP ecosystem is growing fast: catalogs of MCP servers are appearing for databases, file systems, APIs, browsers, and other integrations.

The advantage of MCP is one protocol for all tools. An agent connects to an MCP server and gets access to its tools without writing custom integration code. For more details, see [Chapter 18: Tool Protocols and Tool Servers](../18-tool-protocols-and-servers/README.md).

### A2A Ecosystem

A2A (Agent-to-Agent) is a protocol from Google for inter-agent communication. Each agent publishes an "Agent Card" describing its capabilities. Other agents discover it and send tasks via a standard HTTP API.

A2A solves the interoperability problem: agents from different teams and frameworks interact through a single protocol. For more details, see [Chapter 18: Tool Protocols and Tool Servers](../18-tool-protocols-and-servers/README.md).

### Why This Course Teaches from Scratch

This course builds an agent from scratch for several reasons:

1. **Understanding the foundation.** A framework hides details behind abstractions. When something breaks, you don't know where to look. By writing the agent loop, tool registry, and memory store yourself, you understand every component.

2. **Informed choice.** After implementing from scratch, you know exactly what problems a framework solves. The decision to "use LangGraph" or "write custom" becomes deliberate, not accidental.

3. **Portable knowledge.** Frameworks change. Knowledge of principles (agent loop, Function Calling, context management) transfers to any framework. Knowledge of a specific API does not.

4. **Go as an explicit language.** Most frameworks are written in Python. This course uses Go, which forces you to implement patterns explicitly — no decorator "magic" or metaprogramming.

## Common Errors

### Error 1: Vendor Lock-In

**Symptom:** Your code is tightly coupled to framework API. Changing frameworks requires rewriting everything.

**Cause:** Using framework types directly instead of defining your interfaces.

**Solution:**
```go
// BAD: Direct dependency on framework
func processRequest(frameworkAgent *FrameworkAgent) {
    result := frameworkAgent.Execute(userInput)
}

// GOOD: Interface-based
type Agent interface {
    Execute(input string) (string, error)
}

func processRequest(agent Agent, userInput string) (string, error) {
    return agent.Execute(userInput)
}

// Framework adapter implements your interface
type FrameworkAdapter struct {
    agent *FrameworkAgent
}

func (a *FrameworkAdapter) Execute(input string) (string, error) {
    return a.agent.Execute(input)
}
```

### Error 2: Over-Engineering Custom Runtime

**Symptom:** You spend months building features that frameworks provide out of the box.

**Cause:** Not evaluating if custom implementation is really needed.

**Solution:** Start with framework for prototyping. Extract to custom only when hitting real limitations.

### Error 3: Ignoring Framework Limitations

**Symptom:** You constantly fight the framework, trying to make it do things it wasn't designed for.

**Cause:** Don't understand framework design decisions and limitations.

**Solution:** Read framework documentation carefully. If limitations are too strong, consider custom runtime.

### Error 4: No Migration Path

**Symptom:** You're locked with framework even when it no longer fits your needs.

**Cause:** Tight coupling makes migration impossible without rewriting everything.

**Solution:** Design with interfaces from the start. Keep framework as implementation detail, not main dependency.

## Mini-Exercises

### Exercise 1: Define Tool Interface with JSON Schema

Create an abstract `Tool` interface that works independently of any framework, using JSON Schema for validation:

```go
import (
    "context"
    "encoding/json"
    "github.com/invopop/jsonschema"
    "github.com/xeipuuv/gojsonschema"
)

type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, args map[string]any) (any, error)
    Schema() json.RawMessage
    ValidateArgs(args map[string]any) error
}

// Example implementation with JSON Schema validation
type PingTool struct {
    schema json.RawMessage
}

func (t *PingTool) Name() string {
    return "ping"
}

func (t *PingTool) Description() string {
    return "Ping a host to check connectivity"
}

func (t *PingTool) Schema() json.RawMessage {
    return t.schema
}

func (t *PingTool) ValidateArgs(args map[string]any) error {
    // Use gojsonschema for validation
    schemaLoader := gojsonschema.NewBytesLoader(t.schema)
    documentLoader := gojsonschema.NewGoLoader(args)
    
    result, err := gojsonschema.Validate(schemaLoader, documentLoader)
    if err != nil {
        return err
    }
    
    if !result.Valid() {
        return fmt.Errorf("validation failed: %v", result.Errors())
    }
    
    return nil
}

func (t *PingTool) Execute(ctx context.Context, args map[string]any) (any, error) {
    // Validate before execution
    if err := t.ValidateArgs(args); err != nil {
        return nil, err
    }
    
    // Execute tool...
    return "pong", nil
}
```

**Expected result:**
- Interface doesn't depend on framework
- Can be implemented by any framework adapter
- Provides all necessary information for tool execution
- Includes JSON Schema validation

### Exercise 2: Framework Adapter

Create an adapter that wraps framework tool system to implement your `Tool` interface:

```go
type FrameworkToolAdapter struct {
    frameworkTool FrameworkTool
}

func (a *FrameworkToolAdapter) Name() string {
    // Adapt framework tool name
}

func (a *FrameworkToolAdapter) Execute(ctx context.Context, args map[string]any) (any, error) {
    // Convert your interface to framework API
}
```

**Expected result:**
- Framework becomes implementation detail
- Your code uses your interfaces
- Easy to change frameworks later

### Exercise 3: Decision Matrix

Create a decision matrix for choosing between custom runtime and framework:

| Criterion | Custom Runtime | Framework |
|----------|----------------|-----------|
| Development speed | ? | ? |
| Flexibility | ? | ? |
| Maintenance burden | ? | ? |
| Vendor lock-in risk | ? | ? |

Fill the matrix based on your specific requirements.

**Expected result:**
- Clear understanding of tradeoffs
- Informed decision for your use case

## Completion Criteria / Checklist

**Completed:**
- [x] Understand when to use frameworks vs custom runtime
- [x] Know how to avoid vendor lock-in through interfaces
- [x] Can evaluate frameworks against your requirements
- [x] Understand common patterns in frameworks

**Not completed:**
- [ ] Choosing framework without evaluating requirements
- [ ] Tight coupling to framework API
- [ ] No migration path if framework doesn't fit
- [ ] Ignoring framework limitations

## Connection with Other Chapters

- **[Chapter 09: Agent Anatomy](../09-agent-architecture/README.md)** — Understanding agent components helps evaluate frameworks
- **[Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)** — Tool interfaces are key to portability
- **[Chapter 10: Planning and Workflow Patterns](../10-planning-and-workflows/README.md)** — Frameworks often provide planning patterns
- **[Chapter 18: Tool Protocols and Tool Servers](../18-tool-protocols-and-servers/README.md)** — Standard protocols reduce vendor lock-in
- **[Agent Skills](https://agentskills.io/)** — Open format for agent skills (`SKILL.md`), supported by Cursor, Claude Code, VS Code, and others. See [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#skills)

## What's Next?

After understanding the ecosystem, proceed to:
- **[15. Real-World Case Studies](../15-case-studies/README.md)** — Study examples of real agents

