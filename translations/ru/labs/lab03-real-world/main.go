package main

import (
	"encoding/json"
	"fmt"
)

// 1. Интерфейс Инструмента
type Tool interface {
	Name() string
	Description() string
	// Execute принимает JSON аргументы и возвращает строку-результат
	Execute(args json.RawMessage) (string, error)
}

// 2. Реализация Proxmox Tool
type ProxmoxListVMsTool struct{}

func (t *ProxmoxListVMsTool) Name() string        { return "list_vms" }
func (t *ProxmoxListVMsTool) Description() string { return "List all VMs" }
func (t *ProxmoxListVMsTool) Execute(args json.RawMessage) (string, error) {
	// TODO: Здесь должен быть реальный вызов API Proxmox
	// client.GetNodes()...
	return "VM-100 (Running), VM-101 (Stopped)", nil
}

// 3. Реализация Ansible Tool
type AnsibleRunPlaybookTool struct{}

func (t *AnsibleRunPlaybookTool) Name() string        { return "run_playbook" }
func (t *AnsibleRunPlaybookTool) Description() string { return "Run ansible playbook" }
func (t *AnsibleRunPlaybookTool) Execute(args json.RawMessage) (string, error) {
	// TODO: Парсинг аргументов
	// var params struct { Playbook string }
	// json.Unmarshal(args, &params)
	
	// TODO: exec.Command("ansible-playbook", params.Playbook)
	return "Playbook executed successfully", nil
}

func main() {
	// 4. Реестр инструментов (Map)
	registry := make(map[string]Tool)
	
	pTool := &ProxmoxListVMsTool{}
	registry[pTool.Name()] = pTool

	aTool := &AnsibleRunPlaybookTool{}
	registry[aTool.Name()] = aTool

	fmt.Println("Available tools:", len(registry))

	// 5. Эмуляция вызова (как будто от LLM)
	toolName := "list_vms"
	toolArgs := json.RawMessage("{}")

	if tool, ok := registry[toolName]; ok {
		fmt.Printf("Executing %s...\n", toolName)
		result, _ := tool.Execute(toolArgs)
		fmt.Println("Result:", result)
	} else {
		fmt.Println("Tool not found")
	}
}

