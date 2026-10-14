# Documentation Style Guide

This document describes a unified style for all handbook chapters `book/*`. The goal is to make documentation as simple and educational through practice as the lab assignments in `labs/*/MANUAL.md`.

## Philosophy

**Principle:** Intuition and practice first, then formalization.

- **Simple language** — avoid academic jargon without necessity
- **Through examples** — show how it works in practice
- **Errors as learning** — common mistakes help understand concepts more deeply
- **Self-check** — mini-exercises and checklists for reinforcement

## Unified Chapter Template

Each chapter `book/*/README.md` should follow this structure:

### 1. Why This Chapter? (required)

1-2 paragraphs explaining:
- Why read this chapter
- What problem it solves
- Why it's important for creating agents

**Example:**
```markdown
## Why This Chapter?

Function Calling turns an LLM from a "chatterbox" into a "worker". Without tools, an agent can only respond with text, but can't interact with the real world.
```

### 2. Real-World Case Study (required)

Short scenario from practice showing the problem and solution.

**Format:**
- **Situation:** Problem description
- **Problem:** What doesn't work without this concept
- **Solution:** How the concept solves the problem

**Example:**
```markdown
### Real-World Case Study

**Situation:** User writes: "Check status of server web-01"
**Problem:** The bot can't actually check the server. It only talks.
**Solution:** Function Calling allows the model to call real Go functions.
```

### 3. Theory in Simple Terms (required)

Intuitive explanation of concept without formalization. Use analogies and simple examples.

**Rules:**
- Short paragraphs (2-3 sentences)
- Avoid mathematics in main text
- Use analogies ("like in real life...")

**Example:**
```markdown
## Theory in Simple Terms

### How Does Function Calling Work?

1. You describe a function in JSON Schema format
2. The LLM receives the description and decides: "I need to call this function"
3. The LLM generates JSON with function name and arguments
4. Your code parses the JSON and executes the real function
5. The result is returned to the LLM for further processing
```

### 4. How It Works (Step-by-Step) (required)

Algorithm or work protocol. Show steps with code examples.

**Format:**
- Numbered steps
- Minimal code example at each step
- Comments explain what's happening

**Example:**
```markdown
## Execution Algorithm

### Step 1: Tool Definition

```go
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "get_server_status",
            Description: "Get the status of a server by IP address",
            // ...
        },
    },
}
```

**Important:** `Description` is the most important field! The LLM relies on it.
```

### 5. Mini Code Example (required)

Complete working example showing the concept in action.

**Rules:**
- Start with minimal example
- Then expand, showing more complex cases
- Comments explain each important point

### 6. Common Mistakes (required, minimum 3-5)

Format: **Symptom → Cause → Solution**

**Template:**
```markdown
### Mistake N: [Name]

**Symptom:** [What user sees when something is wrong]

**Cause:** [Why this happens]

**Solution:**
```go
// BAD
// ...

// GOOD
// ...
```
```

**Example:**
```markdown
### Mistake 1: History Not Saved

**Symptom:** Agent doesn't remember previous messages.

**Cause:** You don't add assistant's answer to history.

**Solution:**
```go
// BAD
messages = append(messages, userMessage)
resp := client.CreateChatCompletion(...)
answer := resp.Choices[0].Message.Content
// History not updated!

// GOOD
messages = append(messages, userMessage)
resp := client.CreateChatCompletion(...)
messages = append(messages, resp.Choices[0].Message)  // Save answer!
```
```

### 7. Mini-Exercises / Self-Check (required)

Practical assignments for reinforcing material.

**Format:**
```markdown
## Mini-Exercises

### Exercise 1: [Name]

[Assignment description]

```go
// Code template to start
```

**Expected result:**
- [Criterion 1]
- [Criterion 2]
```

### 8. "Done" Checklist (required)

List of criteria by which readers can check that they understood the material.

**Format:**
```markdown
## Completion Criteria / Checklist

✅ **Completed:**
- [Criterion 1]
- [Criterion 2]

❌ **Not completed:**
- [Common mistake 1]
- [Common mistake 2]
```

### 9. For the Curious (optional, but recommended)

Formalization, mathematics, deep details. Place at the end so as not to break the main flow.

**Format:**
```markdown
## For the Curious

> This section explains [what] at a deeper level. Can skip if you're only interested in practice.

[Formalization, mathematics, implementation details]
```

### 10. Connection with Other Chapters (required)

Links to related chapters and lab assignments.

**Format:**
```markdown
## Connection with Other Chapters

- **[Chapter X: Name](../XX-chapter/README.md)** — [how related]
- **[Lab XX: Name](../../labs/labXX-name/README.md)** — [practice]
```

### 11. What's Next? (required)

Navigation to next chapter.

**Format:**
```markdown
## What's Next?

After studying [topic], proceed to:
- **[XX. Next Chapter](../XX-next/README.md)** — [brief description]
```

## Simple Language Rules

### Sentence Structure

- **Short sentences** (up to 20 words)
- **One fact per sentence**
- **Active voice** ("Agent calls function" instead of "Function is called by agent")

### Terminology

- **Explain in simple terms first**, then use term
- **Term consistency** — use one term for one concept
- **Use technical terms consistently** — establish terminology and stick to it

**Example:**
```markdown
// BAD
Function Calling is a function call mechanism.

// GOOD
Function Calling (function calls) is a mechanism where LLM returns not text, but structured JSON with function name and arguments.
```

### Code Examples

**Rules:**
- **Minimal example first** — show simplest working version
- **Then expand** — add complexity gradually
- **Comments explain "why"**, not "what" (code is self-documenting)
- **Avoid "walls of text"** — if example is long, break into steps

**Example:**
```markdown
// Minimal example
```go
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name: "ping",
            Description: "Ping a host",
        },
    },
}
```

// Extended example with validation
```go
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "ping",
            Description: "Ping a host to check connectivity",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "host": {"type": "string"}
                },
                "required": ["host"]
            }`),
        },
    },
}
```
```

### Magic vs Reality

For explaining complex concepts, use "Magic vs Reality" pattern:

**Format:**
```markdown
### [Concept] — Magic vs Reality

**❌ Magic (how usually explained):**
> [Simplified/incorrect explanation]

**✅ Reality (how it actually works):**

[Detailed explanation with code examples]
```

This helps:
- Dispel myths and simplifications
- Show real mechanism
- Give reader correct understanding

## "Common Mistakes" Format

Each mistake should follow template:

```markdown
### Mistake N: [Short name]

**Symptom:** [What user sees]

**Cause:** [Why this happens]

**Solution:**
```go
// BAD
[Wrong code]

// GOOD
[Correct code]
```
```

**Important:**
- Start with symptom (what user sees)
- Explain cause (why it happens)
- Give concrete solution with code

## "Mini-Exercises" Format

```markdown
## Mini-Exercises

### Exercise N: [Name]

[Assignment description and context]

```go
// Code template (if needed)
```

**Expected result:**
- [Criterion 1]
- [Criterion 2]
```

## "Checklist" Format

```markdown
## Completion Criteria / Checklist

✅ **Completed:**
- [Criterion 1]
- [Criterion 2]

❌ **Not completed:**
- [Common mistake 1]
- [Common mistake 2]
```

## Examples from Different Domains

**Rule:** One main continuous scenario (DevOps) + 1-2 short examples from other domains only where it enhances understanding.

**When to add examples from other domains:**
- When the concept is universal and examples from different domains show different aspects
- When you need to show that the approach works not only in DevOps

**When NOT to add:**
- If examples duplicate each other
- If it complicates reading without benefit

## Navigation Between Chapters

At the end of each chapter:

```markdown
---

**Navigation:** [← Previous Chapter](../XX-prev/README.md) | [Table of Contents](../README.md) | [Next Chapter →](../XX-next/README.md)
```

## Headings

**Structure:**
- `#` — chapter title (only at file start)
- `##` — main sections (Why, Theory, Algorithm, etc.)
- `###` — subsections (Mistake 1, Exercise 1, etc.)
- `####` — only if really needed

**Rules:**
- Headings should be descriptive, not abstract
- Avoid "Introduction", "Conclusion" — use concrete names

## Diagrams (Mermaid)

Use diagrams for visualization:
- Architecture
- Algorithms
- Data flows

**Rules:**
- Diagrams should be simple and clear
- If diagram is complex, break into several simple ones
- Always add text description under diagram

## Links

**Format:**
- To other chapters: `[Name](../XX-chapter/README.md)`
- To labs: `[Lab XX: Name](../../labs/labXX-name/README.md)`
- To sections: `[Section Name](../XX-chapter/README.md#anchor)`

**Rules:**
- Always use relative paths
- Check that links work
- Use descriptive link names, not "here" or "there"

## Pre-Commit Checklist

Before commit, check:

- [ ] Structure matches chapter template
- [ ] Has "Why This Chapter?" section
- [ ] Has real-world case study
- [ ] Has minimum 3 common mistakes
- [ ] Has mini-exercises
- [ ] Has checklist
- [ ] Code examples are minimal and clear
- [ ] No academic jargon without explanation
- [ ] All links work
- [ ] Navigation at end of chapter is correct

## Канон ревизии 2026: актуальный стек и инварианты

Этот раздел фиксирует технические решения ревизии книги. При любых правках глав проверять текст на соответствие этому канону.

### Актуальные модели

В примерах и таблицах использовать только: **Fable 5.1, Opus 5.5, GLM-5.3, GLM-5.3 Flash** (быстрый/дешёвый вариант — GLM-5.3 Flash). Запрещено в тексте глав: gpt-4o-mini, GPT-4o, GPT-3.5-Turbo, Claude 3.x, Hermes-2-Pro, таблицы контекстных окон и цен 2023–2024 годов. Для учебных примеров годится любой из четырёх; «дешёвый на служебные узлы» — GLM-5.3 Flash.

### Примеры кода: нейтральный Go, а не клиент конкретного вендора

- Весь код книги — на **прямом протоколе native tool-calling** (HTTP/JSON к любому провайдеру): конкретные структуры Go, `messages []Message`, `tools []ToolDef`, `resp.Choices[0].Message.ToolCalls`. Без импортов `sashabaranov/go-openai` и без привязки к чужому SDK.
- Цель примера — показать принцип, который читатель повторит с любым провайдером. Провайдеро-зависимые детали (SSE-формат, поле `max_completion_tokens` vs `max_tokens`) — в сносках, не в основном коде.
- Арифметика с токенами — только через `usage.PromptTokens` из ответа провайдера; собственные счётчики (~3 символа/токен) — как fallback-оценка, помеченная как оценка.

### Atlas SDK: где и как упоминается

Atlas SDK — приватный (не open-source). Правила упоминаний:

1. **Сигнатуры SDK — только в одном месте**: appendix «Production view: Atlas SDK» (карта модулей + 8–10 ключевых контрактов + дата актуальности). Нигде больше полных листингов API SDK быть не должно.
2. **В главах — только сайдбары-инварианты** (≤10 строк): «Как это устроено в реальном SDK» без кода SDK — инвариант, а не сигнатура. Примеры инвариантов: «один порог condense ≈0.85, максимум 1 сжатие за Run», «подтверждение человека — обязательный аргумент инструмента, "подтвердил бот" = отказ», «бюджет на результат инструмента (128 KB), иначе контекст съедается», «исчерпание лимита итераций — терминальная ошибка, а не "ещё попытка"».
3. Сигнатуры гниют, инварианты живут. Если врезка хочет показать сигнатуру — она должна уехать в appendix.

### Инварианты, единые для всей книги (терминология и факты)

- **Prompt cache**: system prompt неизменяем; динамика в ход — через tool results и добавление сообщений после префикса, никогда — мутацией `messages[0]`. `time.Now()` и любое «текущее состояние» в system prompt — антипаттерн.
- **Температура**: не «детерминизм». Temperature — настройка качества/разнообразия; воспроизводимость процесса дают структура (schema, валидация), гарды в коде и evals. Формулировка «T=0 делает агента детерминированным» запрещена.
- **Безопасность**: инвариант держит код (allowlist, права хоста, изоляция процесса), промпт объясняет, почему инвариант такой. CAPS-крики и «CRITICAL: ALWAYS…» в промптах — запрещены; мотивированные формулировки работают лучше. Denylist подстрок («ignore previous instructions») — не защита; allowlist — да.
- **Confirm — UX, не security boundary.** Граница — код и изоляция. Подтверждение человека передаётся обязательным аргументом инструмента (например `confirmed_by`); «подтвердил бот» = отказ.
- **Память**: линейная история сообщений; никаких «слоёв», блоков, скоринга сообщений в основном тексте (только как анти-паттерн «для любопытных»). Condense — один механизм: порог ~0.85 окна, максимум одно сжатие за Run, кэшируемый префикс сохраняется.
- **Бюджет контекста**: описание инструментов платится в каждом запросе (ориентир: 4 компактных tools ≈ 742 токена). Число tools и размер описаний — вопрос дизайна №1. Progressive disclosure (скилы) вместо каталогов из 20+ tools-обёрток.
- **Мультиагентность**: по умолчанию — один агентный цикл + скилы. Субагент — исключение: ревью с чистой памятью и вынос шумного чтения; пишущих внешних инструментов у субагента нет.
- **Ошибки**: transient → retry внутри клиента; loop-limit/таймаут → терминально + сообщение человеку. Не подмешивать их в один retry-цикл.
- **Автономный ход**: дедлайн на ход; FSM задач queued → working → (waiting/paused); исходы `working` — asleep (новый ход по упоминанию) или done/failed/cancelled. «Свобода внутри хода, границы снаружи».

### Что резать при правках

- Тройные повторы одного протокола/механизма — канон в одной главе, в остальных ссылки.
- Примеры-«копии лаб» на 200+ строк — заменять ссылкой и diff'ом отличий.
- Выдуманные эвристики как «production-решение» (ModelSelector по стадии, `len(words)<=15`, магические пороги 0.8/0.9) — сжать до оговорки или убрать.

### Скоуп ревизии: лабы

Лабы `labs/` и зеркало `translations/ru/` **ревизованы в рамках 2026**: лабы с LLM-клиентом (00–02, 04–11, 13) переведены на нейтральный клиент `llm` (пакет `llm/` в корне репо, модель через `LLM_MODEL`, дефолт `glm-5.3`) в обеих версиях; lab03 (реестр инструментов) и lab12 (tool-server) LLM-клиента не имеют по дизайну и не менялись. Старый стек (`sashabaranov/go-openai`) остался только внутри адаптера `llm/openai.go` как деталь реализации. Пометки о неревизованности лаб в главах сняты в обеих версиях; новые главы не должны добавлять такие пометки для `labs/`, код глав может опираться на лабы как на образец (канон по-прежнему в главах).

---
