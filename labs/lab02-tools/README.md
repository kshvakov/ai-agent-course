# Lab 02: Function Calling (Tools)

## Goal
Understand the Function Calling mechanism. Learn to describe Go functions so that an LLM can call them.

## Important for Local Models ⚠️
Not all local models support **Function Calling**.
If you're using **LM Studio** or **Ollama**, choose models with tags `function-calling`, `tool-use`, or `agent`.
Good options:
*   `Mistral 7B Instruct`
*   `Hermes 2 Pro`
*   `Llama 3 (some tunes)`
*   `Gorilla OpenFunctions`

If the model doesn't support tools, it may simply continue the conversation with text, ignoring your `Tools` instructions.

## Theory
A regular LLM returns text. But if you describe "Tools" to it in JSON Schema format, it can return a structured function call request.

Process:
1.  **Request:** You send history + function descriptions (Tools Definitions).
2.  **Decision:** Model decides: "Need to call a function".
3.  **Response:** Model returns `ToolCalls` flag (instead of text).
4.  **Execution:** Your code sees this flag and executes the function.

## Assignment
We have a stub function `GetServerStatus(ip string)`.

1.  **Setup:** Initialize the client with `NewClientWithConfig` (like in Lab 01) to work locally.
2.  **Definition:** Describe the function `get_server_status` in `openai.Tool`.
3.  **Request:** Send a request: "Check server status 192.168.1.10".
4.  **Processing:** Check `msg.ToolCalls`. If not empty — print the function name and arguments.
