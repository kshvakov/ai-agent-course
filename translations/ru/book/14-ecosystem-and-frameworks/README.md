# 14. Экосистема и Фреймворки

## Зачем это нужно?

При создании AI-агентов вы сталкиваетесь с выбором: писать всё с нуля или использовать фреймворк? Оба подхода имеют плюсы и минусы, и понимание того, когда что выбирать, критично для долгосрочного успеха.

Эта глава поможет принимать архитектурные решения, избегать vendor lock-in и использовать готовые решения там, где они реально подходят.

### Реальный кейс

**Ситуация:** Вам нужно создать DevOps-агента. Вы можете:
- Использовать популярный фреймворк, который предоставляет всё из коробки
- Построить собственный runtime, адаптированный под ваши нужды

**Проблема:**
- Подход с фреймворком: Быстрый старт, но вы заперты в их абстракциях. Когда нужна кастомная логика, вы боретесь с фреймворком.
- Кастомный подход: Полный контроль, но вы изобретаете велосипед. Каждая функция (tool execution, memory, planning) требует реализации.

**Решение:** Понять компромиссы. Выбирайте фреймворк, когда важна скорость и требования стандартные. Выбирайте кастом, когда нужен специфический контроль или есть уникальные ограничения.

## Теория простыми словами

### Что такое фреймворки для агентов?

Фреймворки для агентов — это библиотеки или платформы, которые предоставляют:
- **Инфраструктуру выполнения инструментов** — обработка function calling, валидация, обработка ошибок
- **Управление памятью** — контекстные окна, саммаризация, сохранение состояния
- **Паттерны планирования** — ReAct циклы, оркестрация workflow, декомпозиция задач
- **Координация multi-agent** — supervisor паттерны, изоляция контекста, маршрутизация

**Суть:** Фреймворки абстрагируют общие паттерны, но они также накладывают ограничения. Понимание этих ограничений помогает решить, когда использовать их, а когда строить кастом.

### Кастомный Runtime vs Фреймворк

**Кастомный Runtime:**
- ✅ Полный контроль над каждым компонентом
- ✅ Нет vendor lock-in
- ✅ Оптимизирован под ваш конкретный use case
- ❌ Больше кода для написания и поддержки
- ❌ Нужно реализовывать общие паттерны самостоятельно

**Фреймворк:**
- ✅ Быстрая разработка, проверенные паттерны
- ✅ Поддержка сообщества и примеры
- ✅ Обрабатывает edge cases, которые вы можете пропустить
- ❌ Меньше гибкости, сложнее кастомизировать
- ❌ Потенциальный vendor lock-in
- ❌ Может включать функции, которые вам не нужны

## Как выбирать?

### Критерии решения

**Выбирайте кастомный Runtime когда:**
1. **Уникальные требования** — Ваш use case не вписывается в стандартные паттерны
2. **Критична производительность** — Нужен тонкий контроль над latency/cost
3. **Минимум зависимостей** — Хотите избежать внешних зависимостей
4. **Цель обучения** — Хотите глубоко понять внутренности
5. **Долгосрочный контроль** — Нужно независимо поддерживать и развивать систему

**Выбирайте фреймворк когда:**
1. **Стандартный use case** — Ваши требования соответствуют общим паттернам
2. **Скорость выхода на рынок** — Нужно быстро запустить
3. **Знакомство команды** — Ваша команда уже знает фреймворк
4. **Быстрое прототипирование** — Исследуете идеи и нужны быстрые итерации
5. **Поддержка сообщества** — Выигрываете от примеров и знаний сообщества

### Соображения портабельности

**Избегайте vendor lock-in через:**
- **Абстракцию интерфейсов** — Определяйте свои интерфейсы для tools, memory, planning
- **Минимальную связь с фреймворком** — Используйте фреймворк для оркестрации, но держите бизнес-логику отдельно
- **Стандартные протоколы** — Предпочитайте стандартные форматы (JSON Schema для tools, OpenTelemetry для observability)
- **Постепенный путь миграции** — Проектируйте так, чтобы можно было менять компоненты позже

### Работа с JSON Schema в Go

При использовании JSON Schema для определений инструментов предпочитайте пакеты Go для валидации и генерации вместо сырого `json.RawMessage`. Это обеспечивает типобезопасность и лучшую обработку ошибок.

**Пример: Использование `github.com/xeipuuv/gojsonschema` для валидации:**

```go
import (
    "github.com/xeipuuv/gojsonschema"
)

// Определяем схему инструмента как JSON Schema
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

// Валидируем аргументы инструмента против схемы
func validateToolArgs(schemaJSON string, args map[string]any) error {
    schemaLoader := gojsonschema.NewStringLoader(schemaJSON)
    documentLoader := gojsonschema.NewGoLoader(args)
    
    result, err := gojsonschema.Validate(schemaLoader, documentLoader)
    if err != nil {
        return fmt.Errorf("ошибка валидации схемы: %w", err)
    }
    
    if !result.Valid() {
        errors := make([]string, 0, len(result.Errors()))
        for _, desc := range result.Errors() {
            errors = append(errors, desc.String())
        }
        return fmt.Errorf("валидация не прошла: %s", strings.Join(errors, "; "))
    }
    
    return nil
}

// Использование при выполнении инструмента
func executePing(args map[string]any) (string, error) {
    // Валидируем аргументы перед выполнением
    if err := validateToolArgs(pingToolSchema, args); err != nil {
        return "", err
    }
    
    host := args["host"].(string)
    count := 4
    if c, ok := args["count"].(float64); ok {
        count = int(c)
    }
    
    // Выполняем ping...
    return fmt.Sprintf("Пропинговали %s %d раз", host, count), nil
}
```

**Пример: Использование `github.com/invopop/jsonschema` для генерации схем:**

```go
import (
    "encoding/json"
    "github.com/invopop/jsonschema"
)

// Определяем параметры инструмента как Go структуру
type PingParams struct {
    Host  string `json:"host" jsonschema:"required,title=Host,description=Hostname or IP address to ping"`
    Count int    `json:"count" jsonschema:"default=4,minimum=1,maximum=10,title=Count,description=Number of ping packets"`
}

// Генерируем JSON Schema из структуры
func generateToolSchema(params any) (json.RawMessage, error) {
    reflector := jsonschema.Reflector{
        ExpandedStruct: true,
        DoNotReference: false,
    }
    
    schema := reflector.Reflect(params)
    schemaJSON, err := json.Marshal(schema)
    if err != nil {
        return nil, fmt.Errorf("не удалось замаршалить схему: %w", err)
    }
    
    return json.RawMessage(schemaJSON), nil
}

// Использование: Генерируем схему для инструмента
func registerPingTool() {
    params := PingParams{}
    schema, err := generateToolSchema(params)
    if err != nil {
        panic(err)
    }
    
    tool := Tool{
        Name:        "ping",
        Description: "Ping a host to check connectivity",
        Schema:      schema, // Используем сгенерированную схему вместо сырого JSON
    }
    
    registry.Register("ping", tool)
}
```

**Преимущества использования пакетов JSON Schema:**
- **Типобезопасность** — Генерируйте схемы из Go структур
- **Валидация** — Валидируйте аргументы перед выполнением инструмента
- **Сообщения об ошибках** — Понятные ошибки валидации
- **Поддерживаемость** — Единый источник истины (Go структура)
- **Документация** — Автоматически генерируемые описания схем

## Общие паттерны во фреймворках

Большинство фреймворков реализуют похожие паттерны:

### Паттерн 1: Tool Registry

```go
// Абстрактный интерфейс (работает с любым фреймворком или кастомом)
type ToolRegistry interface {
    Register(name string, tool Tool) error
    Get(name string) (Tool, error)
    List() []string
}

// Фреймворк может предоставить:
type FrameworkToolRegistry struct {
    tools map[string]Tool
}

func (r *FrameworkToolRegistry) Register(name string, tool Tool) error {
    r.tools[name] = tool
    return nil
}
```

**Суть:** Определяйте свои интерфейсы. Фреймворк становится деталью реализации.

### Паттерн 2: Agent Loop

```go
// Абстрактный интерфейс agent loop
type AgentLoop interface {
    Run(ctx context.Context, input string) (string, error)
    AddTool(tool Tool) error
    SetMemory(memory Memory) error
}

// Ваш код использует интерфейс, а не фреймворк напрямую
func processRequest(agent AgentLoop, userInput string) (string, error) {
    return agent.Run(context.Background(), userInput)
}
```

**Суть:** Dependency injection позволяет менять реализации.

### Паттерн 3: Memory Abstraction

```go
// Абстрактный интерфейс памяти
type Memory interface {
    Store(key string, value any) error
    Retrieve(key string) (any, error)
    Search(query string) ([]any, error)
}

// Память фреймворка реализует ваш интерфейс
type FrameworkMemory struct {
    // Реализация, специфичная для фреймворка
}

func (m *FrameworkMemory) Store(key string, value any) error {
    // Адаптируем API фреймворка к вашему интерфейсу
}
```

**Суть:** Ваши интерфейсы определяют контракт. Фреймворки предоставляют реализации.

## Типовые ошибки

### Ошибка 1: Vendor Lock-In

**Симптом:** Ваш код тесно связан с API фреймворка. Смена фреймворка требует переписывания всего.

**Причина:** Использование типов фреймворка напрямую вместо определения своих интерфейсов.

**Решение:**
```go
// ПЛОХО: Прямая зависимость от фреймворка
func processRequest(frameworkAgent *FrameworkAgent) {
    result := frameworkAgent.Execute(userInput)
}

// ХОРОШО: На основе интерфейсов
type Agent interface {
    Execute(input string) (string, error)
}

func processRequest(agent Agent, userInput string) (string, error) {
    return agent.Execute(userInput)
}

// Адаптер фреймворка реализует ваш интерфейс
type FrameworkAdapter struct {
    agent *FrameworkAgent
}

func (a *FrameworkAdapter) Execute(input string) (string, error) {
    return a.agent.Execute(input)
}
```

### Ошибка 2: Излишняя инженерия кастомного Runtime

**Симптом:** Вы тратите месяцы на построение функций, которые фреймворки предоставляют из коробки.

**Причина:** Не оцениваете, действительно ли нужна кастомная реализация.

**Решение:** Начните с фреймворка для прототипирования. Извлекайте в кастом только когда натыкаетесь на реальные ограничения.

### Ошибка 3: Игнорирование ограничений фреймворка

**Симптом:** Вы постоянно боретесь с фреймворком, пытаясь заставить его делать то, для чего он не был предназначен.

**Причина:** Не понимаете дизайнерские решения и ограничения фреймворка.

**Решение:** Внимательно читайте документацию фреймворка. Если ограничения слишком сильные, рассмотрите кастомный runtime.

### Ошибка 4: Нет пути миграции

**Симптом:** Вы заперты с фреймворком, даже когда он больше не подходит вашим нуждам.

**Причина:** Тесная связь делает миграцию невозможной без переписывания всего.

**Решение:** Проектируйте с интерфейсами с самого начала. Держите фреймворк как деталь реализации, а не основную зависимость.

## Мини-упражнения

### Упражнение 1: Определите интерфейс Tool с JSON Schema

Создайте абстрактный интерфейс `Tool`, который работает независимо от любого фреймворка, используя JSON Schema для валидации:

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

// Пример реализации с валидацией JSON Schema
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
    // Используем gojsonschema для валидации
    schemaLoader := gojsonschema.NewBytesLoader(t.schema)
    documentLoader := gojsonschema.NewGoLoader(args)
    
    result, err := gojsonschema.Validate(schemaLoader, documentLoader)
    if err != nil {
        return err
    }
    
    if !result.Valid() {
        return fmt.Errorf("валидация не прошла: %v", result.Errors())
    }
    
    return nil
}

func (t *PingTool) Execute(ctx context.Context, args map[string]any) (any, error) {
    // Валидируем перед выполнением
    if err := t.ValidateArgs(args); err != nil {
        return nil, err
    }
    
    // Выполняем инструмент...
    return "pong", nil
}
```

**Ожидаемый результат:**
- Интерфейс не зависит от фреймворка
- Может быть реализован любым адаптером фреймворка
- Предоставляет всю необходимую информацию для выполнения инструмента
- Включает валидацию JSON Schema

### Упражнение 2: Адаптер фреймворка

Создайте адаптер, который оборачивает систему инструментов фреймворка для реализации вашего интерфейса `Tool`:

```go
type FrameworkToolAdapter struct {
    frameworkTool FrameworkTool
}

func (a *FrameworkToolAdapter) Name() string {
    // Адаптируем имя инструмента фреймворка
}

func (a *FrameworkToolAdapter) Execute(ctx context.Context, args map[string]any) (any, error) {
    // Конвертируем ваш интерфейс в API фреймворка
}
```

**Ожидаемый результат:**
- Фреймворк становится деталью реализации
- Ваш код использует ваши интерфейсы
- Легко менять фреймворки позже

### Упражнение 3: Матрица решений

Создайте матрицу решений для выбора между кастомным runtime и фреймворком:

| Критерий | Кастомный Runtime | Фреймворк |
|----------|-------------------|-----------|
| Скорость разработки | ? | ? |
| Гибкость | ? | ? |
| Бремя поддержки | ? | ? |
| Риск vendor lock-in | ? | ? |

Заполните матрицу на основе ваших конкретных требований.

**Ожидаемый результат:**
- Чёткое понимание компромиссов
- Обоснованное решение для вашего use case

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Понимаете, когда использовать фреймворки vs кастомный runtime
- Знаете, как избежать vendor lock-in через интерфейсы
- Можете оценить фреймворки против ваших требований
- Понимаете общие паттерны во фреймворках

❌ **Не сдано:**
- Выбор фреймворка без оценки требований
- Тесная связь с API фреймворка
- Нет пути миграции, если фреймворк не подходит
- Игнорирование ограничений фреймворка

## Связь с другими главами

- **[Глава 09: Анатомия Агента](../09-agent-architecture/README.md)** — Понимание компонентов агента помогает оценить фреймворки
- **[Глава 03: Инструменты и Function Calling](../03-tools-and-function-calling/README.md)** — Интерфейсы инструментов — ключ к портабельности
- **[Глава 10: Planning и Workflow-паттерны](../10-planning-and-workflows/README.md)** — Фреймворки часто предоставляют паттерны планирования
- **[Глава 18: Протоколы Инструментов и Tool Servers](../18-tool-protocols-and-servers/README.md)** — Стандартные протоколы уменьшают vendor lock-in

## Что дальше?

После понимания экосистемы переходите к:
- **[15. Кейсы из Реальной Практики](../15-case-studies/README.md)** — Изучите примеры реальных агентов

---

**Навигация:** [← Context Engineering](../13-context-engineering/README.md) | [Оглавление](../README.md) | [Кейсы →](../15-case-studies/README.md)

