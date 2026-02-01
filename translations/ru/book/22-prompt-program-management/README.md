# 22. Prompt и Program Management

## Зачем это нужно?

Вы изменили промпт, и агент стал работать хуже. Но вы не можете понять, что именно изменилось или откатить изменения. Без управления промптами вы не можете:
- Версионировать промпты
- Отслеживать изменения и их влияние
- Тестировать новые версии перед деплоем
- Откатывать плохие изменения

Prompt и Program Management — это контроль над поведением агента. Без него вы не можете безопасно изменять промпты в проде.


### Реальный кейс

**Ситуация:** Вы обновили системный промпт, чтобы улучшить качество ответов. Через день пользователи жалуются, что агент стал хуже работать.

**Проблема:** Нет версионирования промптов и нет evals для проверки изменений. Сложно понять, что именно сломалось, и так же сложно быстро откатиться.

**Решение:** Версионирование промптов в Git, evals для проверки каждой версии и откат при ухудшении метрик. Так вы можете экспериментировать безопасно — и быстро откатывать неудачные изменения.

## Теория простыми словами

### Что такое версионирование промптов?

Версионирование промптов — это хранение всех версий промпта с метаданными (автор, дата, описание изменений). Это позволяет откатить изменения или сравнить версии.

### Что такое промпт-регрессии?

Промпт-регрессии — это ухудшение качества агента после изменения промпта. Evals помогают обнаружить регрессии до деплоя.

## Как это работает (пошагово)

### Шаг 1: Версионирование промптов

Храните промпты в Git или БД с версиями:

```go
type PromptVersion struct {
    ID          string    `json:"id"`
    Version     string    `json:"version"`
    Content     string    `json:"content"`
    Author      string    `json:"author"`
    CreatedAt   time.Time `json:"created_at"`
    Description string    `json:"description"`
}

func getPromptVersion(id string, version string) (*PromptVersion, error) {
    // Загружаем конкретную версию промпта
    // Можно хранить в Git или БД
    return nil, nil
}
```

### Шаг 2: Промпт-регрессии через evals

Используйте evals для проверки каждой версии (см. [Главу 08](../08-evals-and-reliability/README.md)):

```go
func testPromptVersion(prompt PromptVersion) (float64, error) {
    // Запускаем evals для этой версии промпта
    passRate := runEvals(prompt.Content)
    
    // Сравниваем с предыдущей версией
    prevVersion := getPreviousVersion(prompt.ID)
    if prevVersion != nil {
        prevPassRate := runEvals(prevVersion.Content)
        if passRate < prevPassRate {
            return passRate, fmt.Errorf("regression detected: %.2f < %.2f", passRate, prevPassRate)
        }
    }
    
    return passRate, nil
}
```

### Шаг 3: Feature flags

Используйте feature flags для включения/выключения функций без деплоя:

```go
type FeatureFlags struct {
    UseNewPrompt bool
    UseNewModel  bool
}

func getSystemPrompt(flags FeatureFlags) string {
    if flags.UseNewPrompt {
        return getPromptVersion("system", "v2.0").Content
    }
    return getPromptVersion("system", "v1.0").Content
}
```

## Где это встраивать в нашем коде

### Точка интеграции: System Prompt

В `labs/lab06-incident/SOLUTION.md` уже есть SOP в промпте. Версионируйте его:

```go
func getSystemPrompt() string {
    version := os.Getenv("PROMPT_VERSION")
    if version == "" {
        version = "latest"
    }
    
    prompt := getPromptVersion("incident_sop", version)
    return prompt.Content
}
```

## Типовые ошибки

### Ошибка 1: Промпты не версионируются

**Симптом:** После изменения промпта агент стал работать хуже, но вы не можете откатить изменения.

**Решение:** Версионируйте промпты в Git или БД.

### Ошибка 2: Нет evals для проверки изменений

**Симптом:** Изменения промпта деплоятся без проверки, регрессии обнаруживаются только в проде.

**Решение:** Используйте evals для проверки каждой версии перед деплоем.

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Промпты версионируются
- Evals проверяют каждую версию
- Откат при ухудшении метрик

❌ **Не сдано:**
- Промпты не версионируются
- Нет evals для проверки

## Связь с другими главами

- **[Глава 08: Evals и Надежность](../08-evals-and-reliability/README.md)** — Проверка качества промптов
- **[Глава 23: Evals в CI/CD](../23-evals-in-cicd/README.md)** — Автоматическая проверка

---

**Навигация:** [← Workflow и State Management в продакшене](../21-workflow-state-management/README.md) | [Оглавление](../README.md) | [Evals в CI/CD →](../23-evals-in-cicd/README.md)

