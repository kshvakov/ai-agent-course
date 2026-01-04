# Lab 03 Solution: Real World (Interfaces & Infrastructure)

## 🎯 Goal
Learn to build architecture that allows easily adding new tools without changing main code. Use Go interfaces to abstract complex external systems (Proxmox, Ansible).

## 📝 Solution Breakdown

### 1. Command / Interface Pattern
Instead of hardcoding `if name == "func1" ... else if name == "func2"`, we use polymorphism.
The `Tool` interface requires each tool to have:
*   Name (for LLM).
*   Description (for LLM).
*   `Execute` method (for execution).

```go
type Tool interface {
    Name() string
    Description() string
    Execute(args json.RawMessage) (string, error)
}
```

### 2. Ansible Tool Implementation
We create a structure that implements this interface. Inside the `Execute` method, we use standard library `os/exec` to call CLI utility. This is the simplest way to integrate with DevOps tools.

```go
func (t *AnsibleRunPlaybookTool) Execute(args json.RawMessage) (string, error) {
    // 1. Parse arguments
    var params struct { Playbook string }
    if err := json.Unmarshal(args, &params); err != nil {
        return "", err
    }
    
    // 2. Real call (emulated for lab)
    // cmd := exec.Command("ansible-playbook", params.Playbook)
    // ...
    return fmt.Sprintf("Playbook %s executed successfully.", params.Playbook), nil
}
```

### 3. Registry
We use `map[string]Tool` to store all tools. This allows O(1) lookup by name.

### 🔍 Complete Solution Code

```go
package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// --- Interfaces ---

type Tool interface {
	Name() string
	Description() string
	Execute(args json.RawMessage) (string, error)
}

// --- Tools ---

type ProxmoxListVMsTool struct{}

func (t *ProxmoxListVMsTool) Name() string        { return "list_vms" }
func (t *ProxmoxListVMsTool) Description() string { return "List all VMs in the cluster" }
func (t *ProxmoxListVMsTool) Execute(args json.RawMessage) (string, error) {
	// Mock: Real API call would be here
	return "ID: 100, Name: web-01, Status: Running\nID: 101, Name: db-01, Status: Stopped", nil
}

type AnsibleRunPlaybookTool struct{}

func (t *AnsibleRunPlaybookTool) Name() string        { return "run_playbook" }
func (t *AnsibleRunPlaybookTool) Description() string { return "Run ansible playbook" }
func (t *AnsibleRunPlaybookTool) Execute(args json.RawMessage) (string, error) {
	var params struct {
		Playbook string `json:"playbook"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %v", err)
	}
	return fmt.Sprintf("✅ Ansible Playbook '%s' finished successfully.", params.Playbook), nil
}

// --- Main ---

func main() {
	// 1. Tool registration
	registry := make(map[string]Tool)
	
	tools := []Tool{
		&ProxmoxListVMsTool{},
		&AnsibleRunPlaybookTool{},
	}

	for _, t := range tools {
		registry[t.Name()] = t
		fmt.Printf("Registered tool: %s\n", t.Name())
	}

	// 2. Emulate user selection (or LLM)
	// Let's say LLM returned this:
	toolName := "run_playbook"
	toolArgsRaw := json.RawMessage(`{"playbook": "deploy_nginx.yml"}`)

	fmt.Printf("\n🤖 Requesting execution of: %s\n", toolName)

	// 3. Search and execute
	if tool, exists := registry[toolName]; exists {
		result, err := tool.Execute(toolArgsRaw)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
		} else {
			fmt.Printf("📝 Result: %s\n", result)
		}
	} else {
		fmt.Println("❌ Tool not found")
	}
}
```

## 🧠 Why Is This Important?
In large systems, you may have hundreds of tools. Using interfaces and registry allows separating agent logic (brain) from tool logic (hands). You can add new capabilities without rewriting the main agent loop.
