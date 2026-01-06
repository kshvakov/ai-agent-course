# Method Guide: Lab 07 — RAG & Knowledge Base

## Why Is This Needed?

A regular agent only knows what it was taught during training (before cut-off date). It doesn't know your local instructions like "How to restart Phoenix server according to procedure #5".

**RAG (Retrieval Augmented Generation)** is a mechanism for "peeking at a cheat sheet". The agent first searches for information in the knowledge base, then acts.

### Real-World Case Study

**Situation:** User asks: "Restart Phoenix server according to procedure".

**Without RAG:**
- Agent: [Immediately restarts server]
- Result: Procedure violation (should have created backup first)

**With RAG:**
- Agent: Searches knowledge base "Phoenix restart protocol"
- Knowledge base: "POLICY #12: Before restarting Phoenix, you MUST run backup_db"
- Agent: Creates backup → Restarts server → Follows procedure

**Difference:** RAG allows agent to use up-to-date documentation.

## Theory in Simple Terms

### How Does RAG Work?

1. **Task:** "Restart Phoenix server according to procedure"
2. **Agent thought:** "I don't know the procedure. Need to search"
3. **Action:** `search_knowledge_base("Phoenix restart protocol")`
4. **Result:** "File `protocols.txt`: ...first turn off load balancer, then server..."
5. **Agent thought:** "Got it. First turn off load balancer..."

### Simple RAG vs Vector Search

In this lab we implement **simple RAG** (keyword search). In production, **vector search** (Semantic Search) is used, which searches by meaning, not by words.

**Simple RAG (Lab 07):**
```go
// Search by substring match
if strings.Contains(content, query) {
    return content
}
```

**Vector search (production):**
```go
// Documents converted to vectors (embeddings)
// Search similar vectors by cosine distance
similarDocs := vectorDB.Search(queryEmbedding, topK=3)
```

## Execution Algorithm

### Step 1: Creating Knowledge Base

```go
var knowledgeBase = map[string]string{
    "restart_policy.txt": "POLICY #12: Before restarting any server, you MUST run 'backup_db'. Failure to do so is a violation.",
    "backup_guide.txt":   "To run backup, use tool 'run_backup'. It takes no arguments.",
}
```

### Step 2: Search Tool

```go
func searchKnowledge(query string) string {
    var results []string
    for filename, content := range knowledgeBase {
        if strings.Contains(strings.ToLower(content), strings.ToLower(query)) {
            results = append(results, fmt.Sprintf("File: %s\nContent: %s", filename, content))
        }
    }
    if len(results) == 0 {
        return "No documents found."
    }
    return strings.Join(results, "\n---\n")
}
```

### Step 3: System Prompt with Instruction

```go
systemPrompt := `You are a DevOps Agent.
CRITICAL: Always search knowledge base for policies before any restart action.
If you don't know the procedure, search first.`
```

### Step 4: Agent Loop

```go
// Agent receives task: "Restart server"
// Agent thinks: "Need to check procedure"
// Agent calls: search_knowledge_base("restart")
// Gets: "POLICY #12: ...MUST run backup..."
// Agent thinks: "Need to do backup first"
// Agent calls: run_backup()
// Agent calls: restart_server()
```

## Common Errors

### Error 1: Agent Doesn't Search Knowledge Base

**Symptom:** Agent immediately executes action without search.

**Cause:** System Prompt not strict enough.

**Solution:**
```go
// Strengthen prompt:
"BEFORE any action, you MUST search knowledge base. This is mandatory."
```

### Error 2: Search Doesn't Find Documents

**Symptom:** `search_knowledge_base` returns "No documents found".

**Cause:** Query doesn't match document content.

**Solution:**
1. Improve search (use multiple keywords)
2. Add more documents to knowledge base
3. Use vector search (in production)

### Error 3: Agent Ignores Found Information

**Symptom:** Agent found procedure but doesn't follow it.

**Cause:** Information not added to context correctly.

**Solution:**
```go
// Ensure search result is added to history:
messages = append(messages, ChatCompletionMessage{
    Role:    "tool",
    Content: searchResult,  // Agent must see this!
})
```

## Mini-Exercises

### Exercise 1: Improve Search

Implement search by multiple keywords:

```go
func searchKnowledge(query string) string {
    keywords := strings.Fields(query)  // Split into words
    // Search documents containing at least one keyword
    // ...
}
```

### Exercise 2: Add Document Priority

Some documents are more important than others. Implement ranking:

```go
type Document struct {
    Content string
    Priority int  // 1 = high, 3 = low
}
```

## Completion Criteria

✅ **Completed:**
- Agent searches knowledge base before action
- Search finds relevant documents
- Agent follows found instructions
- Code compiles and works

❌ **Not completed:**
- Agent doesn't search knowledge base
- Search doesn't work
- Agent ignores found information

---

**Next step:** After successfully completing Lab 07, proceed to [Lab 08: Multi-Agent](../lab08-multi-agent/README.md)
