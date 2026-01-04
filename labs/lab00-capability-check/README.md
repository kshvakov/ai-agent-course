# Lab 00: Model Capability Benchmark (Diagnostics)

## 🎯 Goal
Before building complex agents, we must scientifically confirm that our model (LLM) possesses the necessary cognitive abilities. In engineering, this is called **Characterization**.

We don't trust labels ("Super-Pro-Max Model"). We trust tests.

## Theory: What Do We Check?

### 1. Instruction Following
The model's ability to strictly adhere to constraints.
*   *Test:* "Write a poem, but don't use the letter 'a'".
*   *Why:* The agent must return strictly defined formats, not "reflections".

### 2. Structured Output (JSON)
The ability to generate valid syntax.
*   *Test:* "Return JSON with fields name and age".
*   *Why:* All interaction with tools is built on JSON. If the model forgets to close a bracket `}`, the agent crashes.

### 3. Function Calling (Tool Usage)
A specific skill of the model to recognize function definitions and generate a special call token.
*   *Why:* Without this, Lab 02 and beyond are impossible.

## Assignment
Run `main.go`. This is an automated test bench. It will run the model through a series of tests and output a report:
*   ✅ Basic Chat
*   ✅ JSON Capability
*   ❌ Function Calling (CRITICAL FAIL) -> **Conclusion: Model is not suitable for Lab 02-08.**

You should run this tool every time you change models (e.g., downloaded a new GGUF in LM Studio).
