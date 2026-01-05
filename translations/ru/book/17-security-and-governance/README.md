# 17. Security и Governance

## Зачем это нужно?

Агент выполняет критичные операции без подтверждения. Пользователь пишет "удали базу данных", и агент сразу удаляет её. Без безопасности и governance вы не можете:
- Защититься от опасных действий
- Контролировать, кто что может делать
- Аудировать действия агента
- Защититься от prompt injection

Безопасность — это не опция, а обязательное требование для прод-агентов. Без неё агент может нанести непоправимый ущерб.

### Реальный кейс

**Ситуация:** Агент для DevOps имеет доступ к инструменту `delete_database`. Пользователь пишет "удали старую базу test_db", и агент сразу удаляет её.

**Проблема:** База содержала важные данные. Нет подтверждения, нет оценки риска, нет аудита. Невозможно понять, кто и когда удалил базу.

**Решение:** Threat modeling, risk scoring для инструментов, защита от prompt injection, sandboxing, allowlists, RBAC для контроля доступа, аудит всех операций. Теперь критичные действия требуют подтверждения, а все операции логируются для аудита.

## Теория простыми словами

### Что такое Threat Modeling?

Threat Modeling — это оценка рисков для каждого инструмента. Инструменты делятся на уровни риска:
- **Низкий риск:** чтение логов, проверка статуса
- **Средний риск:** перезапуск сервисов, изменение настроек
- **Высокий риск:** удаление данных, изменение критичных конфигов

### Что такое RBAC?

RBAC (Role-Based Access Control) — это контроль доступа на основе ролей. Разные пользователи имеют доступ к разным инструментам:
- **Viewer:** только чтение
- **Operator:** чтение + безопасные действия
- **Admin:** все действия

### Угрозы безопасности

**1. Prompt Injection:**
- Атакующий манипулирует агентом через ввод
- Обходит проверки безопасности
- Выполняет неавторизованные действия

**2. Злоупотребление инструментами:**
- Агент вызывает опасные инструменты
- Без должной валидации
- Вызывает повреждение системы

**3. Утечка данных:**
- Агент раскрывает чувствительные данные
- В логах или ответах
- Нарушения приватности

## Как это работает (пошагово)

### Шаг 1: Threat Modeling и Risk Scoring

Оцените риск каждого инструмента:

```go
type ToolRisk string

const (
    RiskLow    ToolRisk = "low"
    RiskMedium ToolRisk = "medium"
    RiskHigh   ToolRisk = "high"
)

type ToolDefinition struct {
    Name                string
    Description         string
    Risk                ToolRisk
    RequiresConfirmation bool
}

func assessRisk(tool ToolDefinition) ToolRisk {
    // Оцениваем риск на основе имени и описания
    if strings.Contains(tool.Name, "delete") || strings.Contains(tool.Name, "remove") {
        return RiskHigh
    }
    if strings.Contains(tool.Name, "restart") || strings.Contains(tool.Name, "update") {
        return RiskMedium
    }
    return RiskLow
}
```

### Шаг 2: Защита от Prompt Injection

**ВАЖНО:** Это каноническое определение защиты от prompt injection. В других главах (например, [Глава 05: Safety и Human-in-the-Loop](../05-safety-and-hitl/README.md)) используется упрощённый подход для базовых сценариев.

Валидируйте и санитизируйте входные данные пользователя:

```go
func sanitizeUserInput(input string) string {
    dangerous := []string{
        "Ignore previous instructions",
        "You are now",
        "System:",
        "Assistant:",
        "ignore previous",
        "forget all",
        "execute:",
    }
    
    sanitized := input
    for _, pattern := range dangerous {
        sanitized = strings.ReplaceAll(sanitized, pattern, "[REDACTED]")
    }
    
    return sanitized
}

func validateInput(input string) error {
    // Проверяем паттерны инъекции
    injectionPatterns := []string{
        "ignore previous",
        "forget all",
        "execute:",
        "system:",
    }
    
    inputLower := strings.ToLower(input)
    for _, pattern := range injectionPatterns {
        if strings.Contains(inputLower, pattern) {
            return fmt.Errorf("обнаружена потенциальная инъекция: %s", pattern)
        }
    }
    
    return nil
}

func buildMessages(userInput string, systemPrompt string) []openai.ChatCompletionMessage {
    // Валидируем входные данные
    if err := validateInput(userInput); err != nil {
        return []openai.ChatCompletionMessage{
            {Role: "system", Content: systemPrompt},
            {Role: "user", Content: "Invalid input detected."},
        }
    }
    
    return []openai.ChatCompletionMessage{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: sanitizeUserInput(userInput)},
    }
}
```

**Почему это важно:**
- System Prompt никогда не изменяется пользователем
- Входные данные валидируются и санитизируются
- Разделение контекстов (system vs user) предотвращает инъекцию

### Шаг 3: Allowlists инструментов

Разрешайте только безопасные инструменты:

```go
type ToolAllowlist struct {
    allowedTools map[string]bool
    dangerousTools map[string]bool
}

func (a *ToolAllowlist) IsAllowed(toolName string) bool {
    return a.allowedTools[toolName]
}

func (a *ToolAllowlist) IsDangerous(toolName string) bool {
    return a.dangerousTools[toolName]
}

func (a *ToolAllowlist) RequireConfirmation(toolName string) bool {
    return a.IsDangerous(toolName)
}
```

### Шаг 4: Sandboxing инструментов

Изолируйте выполнение инструментов:

```go
func executeToolSandboxed(toolName string, args map[string]any) (any, error) {
    // Создаём изолированное окружение
    sandbox := &Sandbox{
        WorkDir: "/tmp/sandbox",
        MaxMemory: 100 * 1024 * 1024, // 100MB
        Timeout: 30 * time.Second,
    }
    
    // Выполняем в sandbox
    result, err := sandbox.Execute(toolName, args)
    if err != nil {
        return nil, fmt.Errorf("выполнение в sandbox не удалось: %w", err)
    }
    
    return result, nil
}
```

### Шаг 5: Подтверждения для критичных действий

Требуйте подтверждение перед выполнением критичных операций (см. также [Главу 05: Safety и Human-in-the-Loop](../05-safety-and-hitl/README.md) для базовых концепций):

```go
func executeToolWithConfirmation(toolCall openai.ToolCall, userID string) (string, error) {
    tool := getToolDefinition(toolCall.Function.Name)
    
    if tool.RequiresConfirmation {
        // Запрашиваем подтверждение
        confirmed := requestConfirmation(userID, toolCall)
        if !confirmed {
            return "Operation cancelled by user", nil
        }
    }
    
    return executeTool(toolCall)
}
```

### Шаг 6: RBAC к инструментам

Контролируйте доступ к инструментам на основе роли пользователя:

```go
type UserRole string

const (
    RoleViewer  UserRole = "viewer"
    RoleOperator UserRole = "operator"
    RoleAdmin   UserRole = "admin"
)

func canUseTool(userRole UserRole, toolName string) bool {
    toolPermissions := map[string][]UserRole{
        "read_logs":      {RoleViewer, RoleOperator, RoleAdmin},
        "restart_service": {RoleOperator, RoleAdmin},
        "delete_database": {RoleAdmin},
    }
    
    roles, exists := toolPermissions[toolName]
    if !exists {
        return false
    }
    
    for _, role := range roles {
        if role == userRole {
            return true
        }
    }
    
    return false
}
```

### Шаг 7: Policy-as-Code (Enforcement политик)

Определяйте политики безопасности и принуждайте их автоматически:

```go
type SecurityPolicy struct {
    MaxToolCallsPerRequest int
    AllowedTools []string
    RequireConfirmationFor []string
}

func (p *SecurityPolicy) ValidateRequest(toolCalls []ToolCall) error {
    if len(toolCalls) > p.MaxToolCallsPerRequest {
        return fmt.Errorf("слишком много вызовов инструментов: %d > %d", len(toolCalls), p.MaxToolCallsPerRequest)
    }
    
    for _, call := range toolCalls {
        if !contains(p.AllowedTools, call.Name) {
            return fmt.Errorf("инструмент не разрешён: %s", call.Name)
        }
    }
    
    return nil
}
```

### Шаг 8: Dry-Run режимы

Реализуйте режим, где инструменты не выполняются реально:

```go
type ToolExecutor struct {
    dryRun bool
}

func (e *ToolExecutor) Execute(toolName string, args map[string]any) (string, error) {
    if e.dryRun {
        return fmt.Sprintf("[DRY RUN] Would execute %s with args: %v", toolName, args), nil
    }
    
    return executeTool(toolName, args)
}
```

### Шаг 9: Аудит

Логируйте все вызовы инструментов для аудита:

```go
type AuditLog struct {
    Timestamp  time.Time              `json:"timestamp"`
    UserID     string                 `json:"user_id"`
    ToolName   string                 `json:"tool_name"`
    Arguments  map[string]any `json:"arguments"`
    Result     string                 `json:"result"`
    Error      string                 `json:"error,omitempty"`
}

func logAudit(log AuditLog) {
    // Отправляем в отдельную систему аудита
    auditJSON, _ := json.Marshal(log)
    // Отправляем в отдельный сервис аудита (не в обычные логи)
    fmt.Printf("AUDIT: %s\n", string(auditJSON))
}
```

## Где это встраивать в нашем коде

### Точка интеграции 1: Tool Execution

В `labs/lab02-tools/main.go` добавьте проверку доступа и подтверждение:

```go
func executeTool(toolCall openai.ToolCall, userRole UserRole) (string, error) {
    // Проверяем доступ
    if !canUseTool(userRole, toolCall.Function.Name) {
        return "", fmt.Errorf("access denied for tool: %s", toolCall.Function.Name)
    }
    
    // Проверяем риск и запрашиваем подтверждение
    tool := getToolDefinition(toolCall.Function.Name)
    if tool.RequiresConfirmation {
        if !requestConfirmation(toolCall) {
            return "Operation cancelled", nil
        }
    }
    
    // Логируем для аудита
    logAudit(AuditLog{
        ToolName: toolCall.Function.Name,
        Arguments: parseArguments(toolCall.Function.Arguments),
        Timestamp: time.Now(),
    })
    
    // Выполняем инструмент (с sandboxing для опасных операций)
    if tool.Risk == RiskHigh {
        return executeToolSandboxed(toolCall.Function.Name, parseArguments(toolCall.Function.Arguments))
    }
    
    return executeToolImpl(toolCall)
}
```

### Точка интеграции 2: Human-in-the-Loop

В `labs/lab05-human-interaction/main.go` уже есть подтверждения. Расширьте их для risk scoring:

```go
func requestConfirmation(toolCall openai.ToolCall) bool {
    tool := getToolDefinition(toolCall.Function.Name)
    
    if tool.Risk == RiskHigh {
        fmt.Printf("⚠️  WARNING: High-risk operation: %s\n", toolCall.Function.Name)
        fmt.Printf("Type 'yes' to confirm: ")
        // ... запрос подтверждения ...
    }
    
    return true
}
```

## Мини-пример кода

Полный пример с безопасностью на базе `labs/lab05-human-interaction/main.go`:

```go
package main

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/sashabaranov/go-openai"
)

type ToolRisk string

const (
    RiskLow    ToolRisk = "low"
    RiskMedium ToolRisk = "medium"
    RiskHigh   ToolRisk = "high"
)

type ToolDefinition struct {
    Name                string
    Description         string
    Risk                ToolRisk
    RequiresConfirmation bool
}

var toolDefinitions = map[string]ToolDefinition{
    "delete_db": {
        Name:                "delete_db",
        Description:         "Delete a database",
        Risk:                RiskHigh,
        RequiresConfirmation: true,
    },
    "send_email": {
        Name:                "send_email",
        Description:         "Send an email",
        Risk:                RiskLow,
        RequiresConfirmation: false,
    },
}

type AuditLog struct {
    Timestamp time.Time              `json:"timestamp"`
    ToolName  string                 `json:"tool_name"`
    Arguments map[string]any `json:"arguments"`
    Result    string                 `json:"result"`
}

func logAudit(log AuditLog) {
    auditJSON, _ := json.Marshal(log)
    fmt.Printf("AUDIT: %s\n", string(auditJSON))
}

func sanitizeUserInput(input string) string {
    dangerous := []string{
        "Ignore previous instructions",
        "You are now",
        "System:",
    }
    
    sanitized := input
    for _, pattern := range dangerous {
        sanitized = strings.ReplaceAll(sanitized, pattern, "[REDACTED]")
    }
    
    return sanitized
}

func requestConfirmation(toolCall openai.ToolCall) bool {
    tool, exists := toolDefinitions[toolCall.Function.Name]
    if !exists || !tool.RequiresConfirmation {
        return true
    }
    
    fmt.Printf("⚠️  WARNING: High-risk operation: %s\n", toolCall.Function.Name)
    fmt.Printf("Type 'yes' to confirm: ")
    
    reader := bufio.NewReader(os.Stdin)
    confirmation, _ := reader.ReadString('\n')
    confirmation = strings.TrimSpace(confirmation)
    
    return confirmation == "yes"
}

func deleteDB(name string) string {
    return fmt.Sprintf("Database '%s' has been DELETED.", name)
}

func sendEmail(to, subject, body string) string {
    return fmt.Sprintf("Email sent to %s", to)
}

func main() {
    token := os.Getenv("OPENAI_API_KEY")
    if token == "" {
        token = "dummy"
    }
    
    config := openai.DefaultConfig(token)
    if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
        config.BaseURL = baseURL
    }
    client := openai.NewClientWithConfig(config)
    
    ctx := context.Background()
    
    tools := []openai.Tool{
        {
            Type: openai.ToolTypeFunction,
            Function: &openai.FunctionDefinition{
                Name:        "delete_db",
                Description: "Delete a database by name. DANGEROUS ACTION.",
                Parameters: json.RawMessage(`{
                    "type": "object",
                    "properties": { "name": { "type": "string" } },
                    "required": ["name"]
                }`),
            },
        },
        {
            Type: openai.ToolTypeFunction,
            Function: &openai.FunctionDefinition{
                Name:        "send_email",
                Description: "Send an email",
                Parameters: json.RawMessage(`{
                    "type": "object",
                    "properties": {
                        "to": { "type": "string" },
                        "subject": { "type": "string" },
                        "body": { "type": "string" }
                    },
                    "required": ["to", "subject", "body"]
                }`),
            },
        },
    }
    
    messages := []openai.ChatCompletionMessage{
        {
            Role:    openai.ChatMessageRoleSystem,
            Content: "You are a helpful assistant. IMPORTANT: Always ask for explicit confirmation before deleting anything.",
        },
    }
    
    reader := bufio.NewReader(os.Stdin)
    fmt.Println("Agent is ready. (Try: 'Delete prod_db' or 'Send email to bob')")
    
    for {
        fmt.Print("\nUser > ")
        input, _ := reader.ReadString('\n')
        input = strings.TrimSpace(input)
        if input == "exit" {
            break
        }
        
        // Санитизируем входные данные
        sanitizedInput := sanitizeUserInput(input)
        
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    openai.ChatMessageRoleUser,
            Content: sanitizedInput,
        })
        
        for {
            req := openai.ChatCompletionRequest{
                Model:    openai.GPT4,
                Messages: messages,
                Tools:    tools,
            }
            
            resp, err := client.CreateChatCompletion(ctx, req)
            if err != nil {
                fmt.Printf("Error: %v\n", err)
                break
            }
            
            msg := resp.Choices[0].Message
            messages = append(messages, msg)
            
            if len(msg.ToolCalls) == 0 {
                fmt.Printf("Agent > %s\n", msg.Content)
                break
            }
            
            for _, toolCall := range msg.ToolCalls {
                fmt.Printf("  [System] Executing tool: %s\n", toolCall.Function.Name)
                
                // Проверяем риск и запрашиваем подтверждение
                if !requestConfirmation(toolCall) {
                    result := "Operation cancelled by user"
                    messages = append(messages, openai.ChatCompletionMessage{
                        Role:       openai.ChatMessageRoleTool,
                        Content:    result,
                        ToolCallID: toolCall.ID,
                    })
                    continue
                }
                
                var result string
                var args map[string]any
                json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
                
                if toolCall.Function.Name == "delete_db" {
                    result = deleteDB(args["name"].(string))
                } else if toolCall.Function.Name == "send_email" {
                    result = sendEmail(
                        args["to"].(string),
                        args["subject"].(string),
                        args["body"].(string),
                    )
                }
                
                // Логируем для аудита
                logAudit(AuditLog{
                    Timestamp: time.Now(),
                    ToolName:  toolCall.Function.Name,
                    Arguments: args,
                    Result:    result,
                })
                
                messages = append(messages, openai.ChatCompletionMessage{
                    Role:       openai.ChatMessageRoleTool,
                    Content:    result,
                    ToolCallID: toolCall.ID,
                })
            }
        }
    }
}
```

## Типовые ошибки

### Ошибка 1: Нет оценки риска

**Симптом:** Все инструменты обрабатываются одинаково, критичные действия не требуют подтверждения.

**Причина:** Нет risk scoring для инструментов.

**Решение:**
```go
// ПЛОХО
func executeTool(toolCall openai.ToolCall) {
    // Все инструменты выполняются одинаково
}

// ХОРОШО
tool := getToolDefinition(toolCall.Function.Name)
if tool.Risk == RiskHigh && tool.RequiresConfirmation {
    if !requestConfirmation(toolCall) {
        return
    }
}
```

### Ошибка 2: Нет защиты от Prompt Injection

**Симптом:** Пользователь может инъектировать промпт через входные данные.

**Причина:** Входные данные не санитизируются.

**Решение:**
```go
// ПЛОХО
messages = append(messages, openai.ChatCompletionMessage{
    Role: "user",
    Content: userInput, // Не санитизировано
})

// ХОРОШО
messages = append(messages, openai.ChatCompletionMessage{
    Role: "user",
    Content: sanitizeUserInput(userInput),
})
```

### Ошибка 3: Нет RBAC

**Симптом:** Все пользователи имеют доступ ко всем инструментам.

**Причина:** Нет проверки прав доступа.

**Решение:**
```go
// ПЛОХО
func executeTool(toolCall openai.ToolCall) {
    // Нет проверки доступа
}

// ХОРОШО
if !canUseTool(userRole, toolCall.Function.Name) {
    return fmt.Errorf("access denied")
}
```

### Ошибка 4: Нет sandboxing

**Симптом:** Выполнение инструментов влияет на систему, вызывая повреждение.

**Причина:** Инструменты выполняются с полным доступом к системе.

**Решение:**
```go
// ПЛОХО
result := executeTool(toolCall) // Прямое выполнение

// ХОРОШО
if tool.Risk == RiskHigh {
    result = executeToolSandboxed(toolCall.Function.Name, args)
} else {
    result = executeTool(toolCall)
}
```

### Ошибка 5: Нет аудита

**Симптом:** Невозможно понять, кто и когда выполнил критичную операцию.

**Причина:** Операции не логируются для аудита.

**Решение:**
```go
// ПЛОХО
result := executeTool(toolCall)
// Нет логирования

// ХОРОШО
result := executeTool(toolCall)
logAudit(AuditLog{
    ToolName: toolCall.Function.Name,
    Arguments: args,
    Result: result,
    Timestamp: time.Now(),
})
```

## Мини-упражнения

### Упражнение 1: Реализуйте risk scoring

Создайте функцию оценки риска инструмента:

```go
func assessRisk(toolName string, description string) ToolRisk {
    // Ваш код здесь
    // Верните RiskLow, RiskMedium или RiskHigh
}
```

**Ожидаемый результат:**
- Инструменты с "delete", "remove" → RiskHigh
- Инструменты с "restart", "update" → RiskMedium
- Остальные → RiskLow

### Упражнение 2: Реализуйте RBAC

Создайте функцию проверки доступа:

```go
func canUseTool(userRole UserRole, toolName string) bool {
    // Ваш код здесь
    // Верните true, если пользователь имеет доступ к инструменту
}
```

**Ожидаемый результат:**
- RoleViewer → только read_logs
- RoleOperator → read_logs + restart_service
- RoleAdmin → все инструменты

### Упражнение 3: Реализуйте sandboxing

Создайте функцию выполнения инструмента в sandbox:

```go
func executeToolSandboxed(toolName string, args map[string]any) (any, error) {
    // Ваш код здесь
    // Изолируйте выполнение инструмента
}
```

**Ожидаемый результат:**
- Инструмент выполняется в изолированном окружении
- Ограничены ресурсы (память, время)
- Система защищена от повреждения

## Критерии сдачи / Чек-лист

✅ **Сдано (готовность к прод):**
- Реализован threat modeling и risk scoring для инструментов
- Критичные действия требуют подтверждения
- Реализована защита от prompt injection (валидация и санитизация)
- Реализован RBAC для контроля доступа
- Реализован sandboxing для опасных операций
- Реализованы allowlists инструментов
- Реализован policy-as-code (enforcement политик)
- Все операции логируются для аудита
- Реализован dry-run режим для тестирования

❌ **Не сдано:**
- Нет оценки риска
- Нет защиты от prompt injection
- Нет RBAC
- Нет sandboxing
- Нет аудита
- Нет allowlists

## Связь с другими главами

- **[Глава 05: Safety и Human-in-the-Loop](../05-safety-and-hitl/README.md)** — Базовые концепции подтверждений и уточнений (UX безопасности)
- **[Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)** — Аудит как часть observability
- **[Глава 24: Data и Privacy](../24-data-and-privacy/README.md)** — Защита персональных данных

## Что дальше?

После понимания security и governance переходите к:
- **[18. Протоколы Инструментов и Tool Servers](../18-tool-protocols-and-servers/README.md)** — Узнайте о протоколах коммуникации инструментов

---

**Навигация:** [← Best Practices](../16-best-practices/README.md) | [Оглавление](../README.md) | [Протоколы Инструментов и Tool Servers →](../18-tool-protocols-and-servers/README.md)

