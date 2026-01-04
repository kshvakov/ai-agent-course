# Lab 00 Solution: Capability Check

## 🎓 Fundamental Explanation

### Why Models "Don't Know" Tools?
LLM (Large Language Model) is a probabilistic text generator. It doesn't "know" about functions.
The **Function Calling** mechanism is the result of special training (Fine-Tuning).
Model developers (OpenAI, Mistral, Meta) add thousands of examples to training data like:
`User: "Check weather" -> Assistant: <special_token>call_tool{"name": "weather"}<end_token>`

If you downloaded "bare" Llama 3 (Base model), it hasn't seen these examples. It will simply continue the dialogue.
If you downloaded Llama 3 Instruct, it's trained to answer questions, but not necessarily trained on OpenAI's tool format.

### Why Temperature 0?
In code you see `Temperature: 0`.
Temperature regulates "randomness" of next token selection.
*   **High Temp (0.8+):** Model chooses less probable words. Good for poetry.
*   **Low Temp (0):** Model always chooses most probable word (ArgMax).
For agents that must output strict JSON or function calls, maximum determinism is needed. Any "creative" error in JSON will break the parser.

### How Does This Test Work?
This is **Unit testing** for a neural network.
In Software Engineering we write tests for code.
In AI Engineering we write **Evals** (Evaluations) for models.
This script is a primitive Eval. In industrial systems (LangSmith, PromptFoo), there are hundreds of such tests.
