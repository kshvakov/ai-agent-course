# Study Guide: Lab 03 — Real World (Interfaces & Infrastructure)

## Why This Lab?

In this laboratory assignment, you'll learn to integrate real infrastructure tools (Proxmox API, Ansible CLI) into agent code. Using interfaces allows making the agent extensible: you can add new tools without changing the main code.

### Real-World Case Study

**Situation:** You've created an agent with tools for Proxmox. Then needed to add Ansible support. You started copying code and changing it manually.

**Problem:** Code became unreadable, adding a new tool requires changes in dozens of places.

**Solution:** Using Go interfaces and Registry pattern allows adding tools without changing the main code.

## Theory in Simple Terms

### Interfaces in Go

An interface defines a contract: "What an object must be able to do", but doesn't say "How to do it".

```go
type Tool interface {
    Name() string
    Description() string
    Execute(args json.RawMessage) (string, error)
}
```

Any type that implements these methods automatically satisfies the `Tool` interface.

### Registry Pattern

Registry is a storage of tools, accessible by name.

```go
registry := make(map[string]Tool)
registry["list_vms"] = &ProxmoxListVMsTool{}
registry["run_playbook"] = &AnsibleRunPlaybookTool{}
```

This allows searching for a tool by name in O(1) and executing it polymorphically.

## Execution Algorithm

### Step 1: Interface Definition

```go
type Tool interface {
    Name() string
    Description() string
    Execute(args json.RawMessage) (string, error)
}
```

### Step 2: Tool Implementation

```go
type ProxmoxListVMsTool struct{}

func (t *ProxmoxListVMsTool) Name() string {
    return "list_vms"
}

func (t *ProxmoxListVMsTool) Description() string {
    return "List all VMs in the Proxmox cluster"
}

func (t *ProxmoxListVMsTool) Execute(args json.RawMessage) (string, error) {
    // Real Proxmox API call logic
    return "VM-100 (Running), VM-101 (Stopped)", nil
}
```

### Step 3: Tool Registration

```go
registry := make(map[string]Tool)

tools := []Tool{
    &ProxmoxListVMsTool{},
    &AnsibleRunPlaybookTool{},
}

for _, tool := range tools {
    registry[tool.Name()] = tool
}
```

### Step 4: Using Registry

```go
toolName := "list_vms"
if tool, exists := registry[toolName]; exists {
    result, err := tool.Execute(json.RawMessage("{}"))
    if err != nil {
        return err
    }
    fmt.Println(result)
}
```

## Common Mistakes

### Mistake 1: Incorrect Interface Implementation

**Symptom:** Compiler complains: "does not implement Tool".

**Cause:** Not all interface methods are implemented.

**Solution:** Ensure all interface methods are implemented:

```go
// All three methods must be present:
func (t *MyTool) Name() string { ... }
func (t *MyTool) Description() string { ... }
func (t *MyTool) Execute(...) (string, error) { ... }
```

### Mistake 2: Tool Not Found in Registry

**Symptom:** `exists == false` when searching for tool.

**Cause:** Tool not registered or name doesn't match.

**Solution:**
```go
// Check that name matches
fmt.Printf("Looking for: %s\n", toolName)
fmt.Printf("Available: %v\n", getKeys(registry))
```

## Mini-Exercises

### Exercise 1: Add New Tool

Create an `SSHCommandTool` that executes a command via SSH:

```go
type SSHCommandTool struct {
    Host string
}

func (t *SSHCommandTool) Execute(args json.RawMessage) (string, error) {
    var params struct {
        Command string `json:"command"`
    }
    json.Unmarshal(args, &params)
    // Implement SSH execution
    return "", nil
}
```

### Exercise 2: Add Validation

Add argument validation before execution:

```go
func (t *AnsibleRunPlaybookTool) Execute(args json.RawMessage) (string, error) {
    var params struct {
        Playbook string `json:"playbook"`
    }
    if err := json.Unmarshal(args, &params); err != nil {
        return "", fmt.Errorf("invalid args: %v", err)
    }
    if params.Playbook == "" {
        return "", fmt.Errorf("playbook is required")
    }
    // ...
}
```

## Completion Criteria

✅ **Completed:**
- `Tool` interface defined
- At least 2 tools implemented (Proxmox and Ansible)
- Tools registered in registry
- Code compiles and works

❌ **Not completed:**
- Interface not fully implemented
- Tools don't register
- Code doesn't compile

---

**Next step:** After successfully completing Lab 03, proceed to [Lab 04: Autonomy](../lab04-autonomy/README.md)
