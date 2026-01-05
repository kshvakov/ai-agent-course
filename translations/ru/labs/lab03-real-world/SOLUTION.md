# Lab 03 Solution: Real World (Interfaces & Infrastructure)

## 🎯 Цель
Научиться строить архитектуру, которая позволяет легко добавлять новые инструменты без изменения основного кода. Использовать интерфейсы Go для абстракции сложных внешних систем (Proxmox, Ansible).

## 📝 Разбор решения

### 1. Паттерн Command / Interface
Вместо хардкода `if name == "func1" ... else if name == "func2"`, мы используем полиморфизм.
Интерфейс `Tool` обязывает каждый инструмент иметь:
*   Имя (для LLM).
*   Описание (для LLM).
*   Метод `Execute` (для выполнения).

```go
type Tool interface {
    Name() string
    Description() string
    Execute(args json.RawMessage) (string, error)
}
```

### 2. Реализация Ansible Tool
Мы создаем структуру, которая реализует этот интерфейс. Внутри метода `Execute` мы используем стандартную библиотеку `os/exec` для вызова CLI утилиты. Это самый простой способ интеграции с DevOps инструментами.

```go
func (t *AnsibleRunPlaybookTool) Execute(args json.RawMessage) (string, error) {
    // 1. Парсим аргументы
    var params struct { Playbook string }
    if err := json.Unmarshal(args, &params); err != nil {
        return "", err
    }
    
    // 2. Реальный вызов (эмуляция для лабы)
    // cmd := exec.Command("ansible-playbook", params.Playbook)
    // ...
    return fmt.Sprintf("Playbook %s executed successfully.", params.Playbook), nil
}
```

### 3. Реестр (Registry)
Мы используем `map[string]Tool` для хранения всех инструментов. Это позволяет искать инструмент по имени за O(1).

### 🔍 Полный код решения

```go
package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// --- Интерфейсы ---

type Tool interface {
	Name() string
	Description() string
	Execute(args json.RawMessage) (string, error)
}

// --- Инструменты ---

type ProxmoxListVMsTool struct{}

func (t *ProxmoxListVMsTool) Name() string        { return "list_vms" }
func (t *ProxmoxListVMsTool) Description() string { return "List all VMs in the cluster" }
func (t *ProxmoxListVMsTool) Execute(args json.RawMessage) (string, error) {
	// Mock: Реальный вызов API был бы здесь
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
	// 1. Регистрация инструментов
	registry := make(map[string]Tool)
	
	tools := []Tool{
		&ProxmoxListVMsTool{},
		&AnsibleRunPlaybookTool{},
	}

	for _, t := range tools {
		registry[t.Name()] = t
		fmt.Printf("Registered tool: %s\n", t.Name())
	}

	// 2. Эмуляция выбора пользователя (или LLM)
	// Допустим, LLM вернула нам это:
	toolName := "run_playbook"
	toolArgsRaw := json.RawMessage(`{"playbook": "deploy_nginx.yml"}`)

	fmt.Printf("\n🤖 Requesting execution of: %s\n", toolName)

	// 3. Поиск и выполнение
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

## 🧠 Почему это важно?
В больших системах у вас могут быть сотни инструментов. Использование интерфейсов и реестра позволяет отделить логику агента (мозга) от логики инструментов (рук). Вы сможете добавлять новые возможности, не переписывая основной цикл агента.

