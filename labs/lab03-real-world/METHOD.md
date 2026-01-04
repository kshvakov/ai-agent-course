# Методическое пособие: Lab 03 — Real World (Interfaces & Infrastructure)

## Зачем это нужно?

В этой лабораторной работе вы научитесь интегрировать реальные инфраструктурные инструменты (API Proxmox, CLI Ansible) в код агента. Использование интерфейсов позволяет сделать агента расширяемым: вы можете добавлять новые инструменты без изменения основного кода.

### Реальный кейс

**Ситуация:** Вы создали агента с инструментами для Proxmox. Потом понадобилось добавить поддержку Ansible. Вы начали копировать код и менять его вручную.

**Проблема:** Код стал нечитаемым, добавление нового инструмента требует правок в десятках мест.

**Решение:** Использование интерфейсов Go и паттерна Registry позволяет добавлять инструменты без изменения основного кода.

## Теория простыми словами

### Интерфейсы в Go

Интерфейс определяет контракт: "Что должен уметь объект", но не говорит "Как это делать".

```go
type Tool interface {
    Name() string
    Description() string
    Execute(args json.RawMessage) (string, error)
}
```

Любой тип, который реализует эти методы, автоматически удовлетворяет интерфейсу `Tool`.

### Паттерн Registry

Registry (Реестр) — это хранилище инструментов, доступное по имени.

```go
registry := make(map[string]Tool)
registry["list_vms"] = &ProxmoxListVMsTool{}
registry["run_playbook"] = &AnsibleRunPlaybookTool{}
```

Это позволяет искать инструмент по имени за O(1) и выполнять его полиморфно.

## Алгоритм выполнения

### Шаг 1: Определение интерфейса

```go
type Tool interface {
    Name() string
    Description() string
    Execute(args json.RawMessage) (string, error)
}
```

### Шаг 2: Реализация инструментов

```go
type ProxmoxListVMsTool struct{}

func (t *ProxmoxListVMsTool) Name() string {
    return "list_vms"
}

func (t *ProxmoxListVMsTool) Description() string {
    return "List all VMs in the Proxmox cluster"
}

func (t *ProxmoxListVMsTool) Execute(args json.RawMessage) (string, error) {
    // Реальная логика вызова API Proxmox
    return "VM-100 (Running), VM-101 (Stopped)", nil
}
```

### Шаг 3: Регистрация инструментов

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

### Шаг 4: Использование реестра

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

## Типовые ошибки

### Ошибка 1: Неправильная реализация интерфейса

**Симптом:** Компилятор ругается: "does not implement Tool".

**Причина:** Не все методы интерфейса реализованы.

**Решение:** Убедитесь, что все методы интерфейса реализованы:

```go
// Должны быть все три метода:
func (t *MyTool) Name() string { ... }
func (t *MyTool) Description() string { ... }
func (t *MyTool) Execute(...) (string, error) { ... }
```

### Ошибка 2: Инструмент не найден в реестре

**Симптом:** `exists == false` при поиске инструмента.

**Причина:** Инструмент не зарегистрирован или имя не совпадает.

**Решение:**
```go
// Проверьте, что имя совпадает
fmt.Printf("Looking for: %s\n", toolName)
fmt.Printf("Available: %v\n", getKeys(registry))
```

## Мини-упражнения

### Упражнение 1: Добавьте новый инструмент

Создайте инструмент `SSHCommandTool`, который выполняет команду через SSH:

```go
type SSHCommandTool struct {
    Host string
}

func (t *SSHCommandTool) Execute(args json.RawMessage) (string, error) {
    var params struct {
        Command string `json:"command"`
    }
    json.Unmarshal(args, &params)
    // Реализуйте SSH выполнение
    return "", nil
}
```

### Упражнение 2: Добавьте валидацию

Добавьте проверку аргументов перед выполнением:

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

## Критерии сдачи

✅ **Сдано:**
- Интерфейс `Tool` определен
- Реализованы минимум 2 инструмента (Proxmox и Ansible)
- Инструменты зарегистрированы в реестре
- Код компилируется и работает

❌ **Не сдано:**
- Интерфейс не реализован полностью
- Инструменты не регистрируются
- Код не компилируется

---

**Следующий шаг:** После успешного прохождения Lab 03 переходите к [Lab 04: Autonomy](../lab04-autonomy/README.md)

