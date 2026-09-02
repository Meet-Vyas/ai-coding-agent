# My Small AI Coding Agent in Go

I built this project to understand what happens behind the scenes when an AI coding assistant reads a file, changes something, or runs a terminal command. The result is a small command-line agent written in Go.

This is not a full replacement for a production coding assistant. It is a focused learning project that helped me understand the basic loop that makes one work.

## What I built

The program takes a prompt from the command line and sends it to a language model through OpenRouter. The model can either answer normally or ask the program to use one of three tools:

- `Read` reads a file and gives its contents back to the model.
- `Write` creates a file or replaces the contents of an existing file.
- `Bash` runs a shell command and gives the output back to the model.

The program keeps the conversation going until the model has enough information to return a final answer. This means a request can take several steps. For example, the model can read a file, understand it, write an updated version, and then explain what it changed.

## How I understand it

The language model does not directly touch my computer. It only replies with either text or a structured request such as “run the `Read` tool with this file path.” My Go program is the part that turns that request into a real action.

The flow looks like this:

1. I give the program a prompt.
2. The program sends the prompt and a description of its available tools to the model.
3. If the model asks for a tool, the program reads the tool name and its JSON arguments.
4. The program performs the action locally.
5. The result is added to the conversation and sent back to the model.
6. This repeats until the model replies without asking for another tool.

The tool-call ID was an important detail for me. It acts like a receipt number: when the program sends a tool result back, the ID tells the model which request that result belongs to. This matters when the model asks for more than one tool in the same response.

## What I learned

I started with a simple API call that sent one prompt and printed one response. From there, I learned how to turn it into an agent instead of a one-off chatbot.

- I learned that tools are described with JSON schemas. The schema tells the model what a tool is called, what it does, and which information it must provide.
- I learned how to decode model-generated JSON into Go structs so the program can safely access fields such as a file path, file contents, or a shell command.
- I learned why an agent needs a loop. A useful answer may depend on several actions, so stopping after the first model response or first tool call is not enough.
- I learned that every assistant message and tool result must be added to the same conversation. Without that history, the model would lose track of what it requested and what happened.
- I learned to handle multiple tool calls instead of assuming the model will request only one action at a time.
- I learned that command failures are also useful information. The Bash tool returns the error and command output to the model, which gives it a chance to understand the problem and try a different approach.
- I learned the practical difference between normal output and error output. The final answer goes to standard output, while failures and diagnostic messages go to standard error.
- Most importantly, I learned that the model decides *what it wants to do*, but ordinary code still decides *what it is actually allowed to do*.

## Running it

You need Go 1.26 or newer and an OpenRouter API key.

```sh
export OPENROUTER_API_KEY="your-api-key"
./your_program.sh -p "Read README.md and summarize it"
```

The program uses `https://openrouter.ai/api/v1` by default. A different OpenAI-compatible endpoint can be supplied with `OPENROUTER_BASE_URL`:

```sh
export OPENROUTER_BASE_URL="https://example.com/v1"
```

The model is currently set to `anthropic/claude-haiku-4.5` in `app/main.go`.

## A safety note

This project deliberately gives the model powerful local tools. `Write` can replace files, and `Bash` can run any command accepted by the shell. There is no sandbox, path restriction, or confirmation step in this version. I only run it in a folder where I am comfortable with the model reading and changing files, and I avoid using it with untrusted prompts.

That limitation was another useful lesson: making a tool work is only the first part. A production-quality agent also needs permission checks, restricted paths and commands, timeouts, clearer error handling, and tests.
