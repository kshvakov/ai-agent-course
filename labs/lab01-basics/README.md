# Lab 01: Hello, LLM! (Basics & Memory)

## Goal
Learn how to work with the OpenAI API (or compatible local API) in Go, and implement a basic chat loop and memory mechanism (History).

## Theory
Any communication with an LLM (ChatGPT, Llama 3) is a stateless process. The model doesn't remember what you wrote a second ago. To create the illusion of dialogue, we send **all** previous messages (history) with each request.

Message structure is usually:
*   `System`: Instruction for the role ("You are a DevOps engineer...").
*   `User`: User's question ("How are you?").
*   `Assistant`: Model's response.

## Task
In the `main.go` file you'll find a console chat template.

1.  **Initialization:** Load the connection settings with `llm.LoadConfig()` (env vars `LLM_API_KEY` / `LLM_BASE_URL` / `LLM_MODEL`, usually sourced from `.env`) and create a client with `llm.NewOpenAI(...)`.
2.  **Memory Loop:** Implement the loop:
    *   Read user input.
    *   Add user message to history (`messages`).
    *   Send ALL history to API.
    *   Get response, display on screen.
    *   Add assistant's response to history.
3.  **System Prompt:** Add a system message at the start of history that sets the role: *"You are an experienced Linux administrator. Answer briefly and to the point."*

## Running with Local Model (LM Studio)
1.  Start LM Studio -> Start Server (Port 1234).
2.  In terminal:
```bash
export LLM_API_KEY="lm-studio"
export LLM_BASE_URL="http://localhost:1234/v1"
# export LLM_MODEL="glm-5.3"  # optional, default glm-5.3
go run main.go
```
(The variables can live in a `.env` file in the repo root instead: `set -a; source ../../.env; set +a`.)
