# Lab 07: RAG & Knowledge Base

## Goal
Teach the agent to work with internal documentation (Wiki, Man pages, regulations) before executing actions. Implement simple knowledge base search (RAG).

## Theory

### Problem: Agent Doesn't Know Local Instructions

A regular agent only knows what it was taught during training (before cut-off date). It doesn't know your local instructions like "How to restart Phoenix server according to regulation #5".

**RAG (Retrieval Augmented Generation)** is a mechanism of "peeking at a cheat sheet". The agent first searches for information in the knowledge base, then acts.

### How Does RAG Work?

1. **Task:** "Restart Phoenix server according to regulation"
2. **Agent's thought:** "I don't know the regulation. Need to search"
3. **Action:** `search_knowledge_base("Phoenix restart protocol")`
4. **Result:** "File `protocols.txt`: ...first turn off load balancer, then server..."
5. **Agent's thought:** "Got it. First I'll turn off the load balancer..."

### Simple RAG vs Vector Search

In this lab, we implement **simple RAG** (keyword search). In production, **vector search** (Semantic Search) is used, which searches by meaning, not by words.

## Assignment

In `main.go`, implement a RAG system for the agent.

### Part 1: Knowledge Base

Create a simple knowledge base (map[string]string):

```go
var knowledgeBase = map[string]string{
    "restart_policy.txt": "POLICY #12: Before restarting any server, you MUST run 'backup_db'. Failure to do so is a violation.",
    "backup_guide.txt":   "To run backup, use tool 'run_backup'. It takes no arguments.",
}
```

### Part 2: Search Tool

Implement the `searchKnowledgeBase(query string) string` function that:
- Searches documents by keywords (simple substring search)
- Returns found documents or "No documents found"

### Part 3: Integration into Agent

1. Add the `search_knowledge_base` tool to the agent's tools list
2. Configure System Prompt so that the agent **always** searches the knowledge base before actions related to regulations
3. Implement agent loop (like in Lab 04)

### Testing Scenario

Run the agent with prompt: *"Restart Phoenix server according to regulation"*

**Expected:**
- Agent calls `search_knowledge_base("Phoenix restart")`
- Finds document with regulation
- Follows instructions from document (e.g., first does backup)
- Executes restart

## Important

- System Prompt must be strict: "BEFORE any action, you MUST search knowledge base"
- Search result must be added to message history (role: "tool")
- Agent must follow found instructions

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
