# 14. Ecosystem and Frameworks

## Why does this matter?

When building AI agents, you face a choice: write everything from scratch or use a framework? Both approaches have pros and cons, and understanding when to pick which is critical for long-term success.

This chapter will help you make architectural decisions, avoid vendor lock-in, and use off-the-shelf solutions where they genuinely fit.

### A real-world case

**Situation:** You need to build a DevOps agent. You can:
- Use a popular framework that provides everything out of the box
- Build your own runtime tailored to your needs

**Problem:**
- Framework approach: Fast start, but you are locked into their abstractions. When you need custom logic, you fight the framework.
- Custom approach: Full control, but you reinvent the wheel. Every feature (tool execution, memory, planning) requires implementation.

**Solution:** Understand the trade-offs. Choose a framework when speed matters and requirements are standard. Choose custom when you need specific control or have unique constraints.

## The theory in plain words

### What are agent frameworks?

Agent frameworks are libraries or platforms that provide:
- **Tool execution infrastructure** — function calling handling, validation, error handling
- **Memory management** — context windows, summarization, state persistence
- **Planning patterns** — ReAct loops, workflow orchestration, task decomposition
- **Multi-agent coordination** — supervisor patterns, context isolation, routing

**The essence:** Frameworks abstract common patterns, but they also impose constraints. Understanding those constraints helps you decide when to use them and when to build custom.

### Custom runtime vs framework

**Custom runtime:**
- [x] Full control over every component
- [x] No vendor lock-in
- [x] Optimized for your specific use case
- [ ] More code to write and maintain
- [ ] You implement common patterns yourself

**Framework:**
- [x] Fast development, proven patterns
- [x] Community support and examples
- [x] Handles edge cases you might miss
- [ ] Less flexibility, harder to customize
- [ ] Potential vendor lock-in
- [ ] May include features you don't need

## How to Choose?

### Decision criteria

**Choose a custom runtime when:**
1. **Unique requirements** — Your use case doesn't fit standard patterns
2. **Performance is critical** — You need fine-grained control over latency/cost
3. **Minimal dependencies** — You want to avoid external dependencies
4. **Learning goal** — You want to deeply understand the internals
5. **Long-term control** — You need to maintain and evolve the system independently

**Choose a framework when:**
1. **Standard use case** — Your requirements match common patterns
2. **Time to market** — You need to ship quickly
3. **Team familiarity** — Your team already knows the framework
4. **Rapid prototyping** — You're exploring ideas and need fast iterations
5. **Community support** — You benefit from community examples and knowledge

### Portability considerations

Vendor lock-in in Go is solved by a familiar pattern — **provider drivers registered via a blank import, as in `database/sql`**. The application depends on a small facade (`Client.Chat` / `Client.Stream`), while the concrete provider is plugged in by importing a driver and selected by a name string in the configuration. Switching providers means changing one line, not rewriting code.

Other things that help you avoid locking yourself in:
- **Standard protocols** — JSON Schema for tools (ch. 03), MCP/A2A for external integrations (ch. 18), OpenTelemetry for observability
- **Minimal coupling to the framework** — if you do take a framework, keep business logic separate from orchestration
- **A gradual migration path** — design so that components can be swapped later

> **How the real SDK does it.** Commercial Go SDKs for LLMs are built exactly this way: eight protocol drivers (Anthropic, OpenAI, Google, and others), an OpenAI-compatible driver serving several providers at once, and a model catalog living in a separate package. The application sees only the facade.

### JSON schema and tools

A tool schema is part of the function calling protocol: the LLM receives it in the tool's description and generates arguments based on it. The format is standard JSON Schema (`type`, `properties`, `required`, `enum`); chapter 03 shows it in full, so we won't repeat it here.

Rule of thumb: **don't write schemas by hand**. The SDK or your runtime generates the schema from the Go parameter struct via reflection — the struct remains the single source of truth, and the schema and code can never drift apart:

```go
type DeployParams struct {
    Service string `json:"service" jsonschema:"description=Service name"`
    Env     string `json:"env"     jsonschema:"enum=staging|production"`
}

// schema := reflector.Reflect(DeployParams{}) → a ready inputSchema for the tool
```

Manual JSON Schema validation is justified only at the boundary with external, untrusted data — for example, when arguments arrive over the network from someone else's agent. For your own tools, `json.Unmarshal` into a Go struct is enough: it will itself report type mismatches and missing required fields.

## Patterns Common to All Frameworks

Under the hood, all frameworks implement the same mechanisms you have already built in this course:

| Pattern | What it is | Where in the course |
|---------|---------|-------------|
| **Tool Registry** | A `name → tool` map plus call dispatch | [Ch. 03](../03-tools-and-function-calling/README.md) |
| **Agent Loop** | The "LLM → tool call → result → LLM" loop until the final answer | [Ch. 04](../04-autonomy-and-loops/README.md) |
| **Message History** | A linear list of messages as the agent's memory | [Ch. 12](../12-agent-memory/README.md) |

A separate warning concerns the **memory abstraction** (a `Memory` with `Store`/`Retrieve`/`Search` methods) that frameworks often offer in the name of "portability." In practice this is an anti-pattern: an agent's memory is a linear message history plus `condense` on context overflow ([ch. 12](../12-agent-memory/README.md)). A universal memory interface solves the problem of switching frameworks — a problem you don't have — and fails to solve the context overflow problem, which you do have. If a framework imposes such an abstraction, check its memory model before adopting it, not after.

## Frameworks and the Ecosystem

### A survey of real frameworks

> **Disclaimer:** agent frameworks evolve quickly — major releases come out every few months, APIs break, strengths and weaknesses shift. The summary below is a snapshot as of the time of writing. Before choosing, check the latest documentation, recent releases, and real production experience (issue trackers, downgrade reports).

**LangGraph (Python, LangChain).** Graph-based agents: every step is a graph node, and transitions are defined by conditions. Suited to complex workflows with branching and loops.

**CrewAI (Python).** Multi-agent systems: agents are grouped into "crews" with roles and tasks. Convenient for scenarios where several agents collaborate toward a shared goal.

**AutoGen (Microsoft, Python).** Multi-agent systems with an emphasis on dialogue between agents. In v0.4 it was rewritten from scratch (a new core plus the AgentChat framework on top) — treat pre-v0.4 materials and tutorials with caution: it is effectively a different framework.

**Semantic Kernel (Microsoft, .NET/Python).** An orchestrator that integrates LLMs with existing code through "plugins." Aimed at the enterprise and the Microsoft ecosystem.

**OpenAI Agents SDK (Python/JS).** A lightweight SDK: an agent as a loop with tools, handoffs between agents, guardrails.

**Google ADK (Python/Java).** Google's Agent Development Kit, tightly integrated with Vertex AI.

**pydantic-ai (Python).** Typed agents: tool schemas are generated from pydantic models — the same idea as generating from a Go struct in [ch. 03](../03-tools-and-function-calling/README.md).

**Vercel AI SDK (TypeScript).** Agent primitives for the web stack.

The list is not exhaustive: the market changes faster than books — verify currency at the time of reading.

### Comparison table

| Framework | Language | Strengths | Weaknesses |
|-----------|------|----------------|---------------|
| **LangGraph** | Python | Graph-based workflows, flexible state, streaming | Complex API, steep learning curve |
| **CrewAI** | Python | Simple multi-agent coordination, roles | Less flexible for non-standard patterns |
| **AutoGen** | Python | Conversational multi-agent systems, Microsoft backing | Heavyweight, hard to debug; v0.4 is effectively a new framework |
| **Semantic Kernel** | .NET, Python | Enterprise-ready, Azure integration | Oriented toward the Microsoft ecosystem |

### What about Go?

There is no dominant agent framework in Go — and that's fine. The working choice of most Go teams: small facade libraries for LLM providers (the driver pattern described above) plus their own agent loop of 300–500 lines, which you write in [chapter 04](../04-autonomy-and-loops/README.md). It's a boring and verifiable solution: all the agent code is yours, debugging is ordinary, dependencies are minimal. A framework is justified when you specifically need its ready-made orchestration patterns, not "to avoid writing the loop by hand."

### The MCP ecosystem

MCP (Model Context Protocol) is an open protocol for connecting tools to LLM agents: one protocol, and any MCP server works with any MCP client. The ecosystem of ready-made servers (databases, file systems, APIs, browsers) is growing fast. The protocol itself is covered in [Chapter 18: Tool Protocols and Tool Servers](../18-tool-protocols-and-servers/README.md).

### The A2A ecosystem

A2A (Agent-to-Agent) is an open protocol for agents to interact with each other: each agent publishes a discovery document describing its capabilities, and others find it and delegate tasks through a standard HTTP API. Details in [Chapter 18](../18-tool-protocols-and-servers/README.md).

### Skills

A separate layer of the ecosystem is skills (Skills, [agentskills.io](https://agentskills.io/)): knowledge packages in the `SKILL.md` format with progressive disclosure — the agent loads an instruction only when a task calls for it. Supported by Cursor, Claude Code, VS Code, and others. Covered in detail in [Chapter 09: Anatomy of an Agent](../09-agent-architecture/README.md#skills).

### Why this course teaches from scratch

This course builds an agent from scratch for several reasons:

1. **Understanding the foundation.** A framework hides details behind abstractions. When something breaks, you don't know where to look. Having written the agent loop, tool registry, and message history with your own hands, you understand every component.

2. **Informed choices.** After implementing things from scratch, you know exactly which problems a framework solves. The decision "use LangGraph" or "write custom" becomes justified rather than accidental.

3. **Portability of knowledge.** Frameworks change. Knowing the principles (agent loop, function calling, context management) transfers to any framework. Knowing a specific API does not.

4. **Go as an explicit language.** Most frameworks are written in Python. This course uses Go, which forces you to implement patterns explicitly — without the "magic" of decorators and metaprogramming.

5. **Simplicity over fashion.** Many "advanced" patterns from agent reviews and blog posts turn out to be unnecessary in practice. A linear message history, a single `condense` on context overflow, a fixed system prompt, an iteration limit, repetition defense — that's enough for most tasks (see [ch. 12](../12-agent-memory/README.md), [ch. 13](../13-context-engineering/README.md)). Multi-level condensation, message importance scoring, dynamic prompts, and heavy "memory frameworks" usually solve the wrong problem: they are expensive, break the prompt cache, and complicate debugging, while rarely paying off under real workloads. This doesn't mean they are always harmful — but before adopting them, it's worth verifying the basic mechanisms and making sure the simple solution really isn't enough. When you write the runtime yourself, this difference is immediately visible.

## Common Mistakes

### Mistake 1: Vendor lock-in

**Symptom:** Your code is tightly coupled to the framework's API. Switching frameworks requires rewriting everything.

**Cause:** Using framework types directly instead of defining your own interfaces.

**Solution:**
```go
// BAD: Direct dependency on the framework
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

// A framework adapter implements your interface
type FrameworkAdapter struct {
    agent *FrameworkAgent
}

func (a *FrameworkAdapter) Execute(input string) (string, error) {
    return a.agent.Execute(input)
}
```

### Mistake 2: Over-engineering a custom runtime

**Symptom:** You spend months building features that frameworks provide out of the box.

**Cause:** You don't evaluate whether a custom implementation is really needed.

**Solution:** Start with a framework for prototyping. Extract into custom code only when you hit real limitations.

### Mistake 3: Ignoring framework limitations

**Symptom:** You constantly fight the framework, trying to make it do what it was never designed for.

**Cause:** You don't understand the framework's design decisions and limitations.

**Solution:** Read the framework documentation carefully. If the limitations are too severe, consider a custom runtime.

### Mistake 4: No migration path

**Symptom:** You are locked in with a framework even when it no longer fits your needs.

**Cause:** Tight coupling makes migration impossible without rewriting everything.

**Solution:** Design with interfaces from the start. Keep the framework as an implementation detail, not the primary dependency.

## Exercises

### Exercise 1: The tool interface

Define a framework-independent `Tool` interface. The schema is generated from the Go parameter struct — no hand-written JSON needed:

```go
type Tool interface {
    Name() string
    Description() string
    Schema() json.RawMessage // generated from the parameter struct (reflection)
    Execute(ctx context.Context, args map[string]any) (any, error)
}

type PingParams struct {
    Host string `json:"host" jsonschema:"description=Hostname or IP"`
}

// The schema comes from the struct; in Execute the arguments are unpacked into it via json.Unmarshal
```

**Expected outcome:**
- The interface is independent of any framework; a framework adapter can implement it
- The schema comes from a Go struct, not hand-written JSON (single source of truth)
- Argument validation is done via `json.Unmarshal` into the parameter struct

### Exercise 2: A framework adapter

Create an adapter that wraps a framework's tool system to implement your `Tool` interface:

```go
type FrameworkToolAdapter struct {
    frameworkTool FrameworkTool
}

func (a *FrameworkToolAdapter) Name() string {
    // Adapt the framework tool's name
}

func (a *FrameworkToolAdapter) Execute(ctx context.Context, args map[string]any) (any, error) {
    // Convert your interface to the framework's API
}
```

**Expected outcome:**
- The framework becomes an implementation detail
- Your code uses your interfaces
- Frameworks are easy to swap later

### Exercise 3: Decision matrix

Create a decision matrix for choosing between a custom runtime and a framework:

| Criterion | Custom Runtime | Framework |
|----------|-------------------|-----------|
| Development speed | ? | ? |
| Flexibility | ? | ? |
| Maintenance burden | ? | ? |
| Vendor lock-in risk | ? | ? |

Fill in the matrix based on your specific requirements.

**Expected outcome:**
- A clear understanding of the trade-offs
- A justified decision for your use case

## Acceptance Criteria / Checklist

**Done:**
- [x] You understand when to use frameworks vs a custom runtime.
- [x] You know how to avoid vendor lock-in: a `database/sql`-style provider facade and your own interfaces (Tool).
- [x] You can evaluate a framework against your requirements and stay calm about loud promises of "revolutionary" patterns.
- [x] You understand the patterns common to all frameworks (Tool Registry, Agent Loop, message history).
- [x] When choosing a framework, you check data freshness (releases, issues, real production feedback).

**Not done:**
- [ ] Choosing a framework without evaluating requirements.
- [ ] Tight coupling to the framework's API (framework types all over the business logic).
- [ ] No migration path if the framework turns out to be a poor fit.
- [ ] Ignoring framework limitations ("I'll make it do what it wasn't designed for").
- [ ] Blindly copying "advanced" patterns from other people's repositories and blog posts without understanding what specific problem they solve.

## Relation to Other Chapters

- **[Chapter 09: Anatomy of an Agent](../09-agent-architecture/README.md)** — Understanding agent components helps you evaluate frameworks.
- **[Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)** — Tool interfaces are the key to portability.
- **[Chapter 10: Planning and Workflow Patterns](../10-planning-and-workflows/README.md)** — Frameworks often provide planning patterns.
- **[Chapter 12: Agent Memory Systems](../12-agent-memory/README.md)** and **[Chapter 13: Context Engineering](../13-context-engineering/README.md)** — The simple memory and context model against which you should measure any framework before taking it on as an "advanced" solution.
- **[Chapter 18: Tool Protocols and Tool Servers](../18-tool-protocols-and-servers/README.md)** — Standard protocols (MCP, A2A) reduce vendor lock-in.
- **[Agent Skills](https://agentskills.io/)** — An open format for agent skills (`SKILL.md`), supported by Cursor, Claude Code, VS Code, and others. See [Chapter 09: Anatomy of an Agent](../09-agent-architecture/README.md#skills).

## What's Next?

Having understood the ecosystem, move on to:
- **[15. Case Studies from Real Practice](../15-case-studies/README.md)** — Study examples of real agents
