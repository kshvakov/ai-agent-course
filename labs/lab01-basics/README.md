# Lab 01: Hello, LLM! (Basics & Memory)

## Goal
Learn to work with OpenAI API (or compatible local API) in Go, implement a basic chat loop and memory mechanism (History).

## Theory
Any communication with an LLM (ChatGPT, Llama 3) is a stateless process. The model doesn't remember what you wrote a second ago. To create the illusion of dialogue, we send the **entire** list of previous messages (history) every time.

Message structure is usually:
*   `System`: Role instruction ("You are a DevOps engineer...").
*   `User`: User's question ("How are you?").
*   `Assistant`: Model's response.

## Assignment
In the `main.go` file, you'll find a console chat skeleton.

1.  **Initialization:** Create an OpenAI client. If `OPENAI_BASE_URL` variable is set (for LM Studio/Ollama), use it.
2.  **Memory Loop:** Implement the loop:
    *   Read user input.
    *   Add user message to history (`messages`).
    *   Send ENTIRE history to API.
    *   Get response, print to screen.
    *   Add assistant's response to history.
3.  **System Prompt:** Add a system message at the start of history that sets the role: *"You are an experienced Linux administrator. Answer briefly and to the point."*

## Running with Local Model (LM Studio)
1.  Start LM Studio -> Start Server (Port 1234).
2.  In terminal:
```bash
export OPENAI_BASE_URL="http://localhost:1234/v1"
export OPENAI_API_KEY="lm-studio"
go run main.go
```
