package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func main() {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseUrl := os.Getenv("OPENROUTER_BASE_URL")
	if baseUrl == "" {
		baseUrl = "https://openrouter.ai/api/v1"
	}

	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))

	ctx := context.Background()

	params := openai.ChatCompletionNewParams{
		Model: "anthropic/claude-haiku-4.5",
		Messages: []openai.ChatCompletionMessageParamUnion{
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(prompt),
					},
				},
			},
		},
		Tools: []openai.ChatCompletionToolUnionParam{
			{
				OfFunction: &openai.ChatCompletionFunctionToolParam{
					Function: openai.FunctionDefinitionParam{
						Name: "Read",
						Description: openai.String("Read and return the contents of a file"),
						Parameters: openai.FunctionParameters{
							"type": "object",
							"properties": map[string]any{
								"file_path": map[string]any{
									"type": "string",
									"description": "The path to the file to read",
								},
							},
							"required": []string{"file_path"},
						},
					},
				},
			},
			{
				OfFunction: &openai.ChatCompletionFunctionToolParam{
					Function: openai.FunctionDefinitionParam{
						Name: "Write",
						Description: openai.String("Write content to a file"),
						Parameters: openai.FunctionParameters{
							"type": "object",
							"properties": map[string]any{
								"file_path": map[string]any{
									"type": "string",
									"description": "The path of the file to write to",
								},
								"content": map[string]any{
									"type": "string",
									"description": "The content to write to the file",
								},
							},

							// Both values are required because Write cannot execute
							// without knowing the destination and its new contents.
							"required": []string{"file_path", "content"},
						},
					},
				},
			},
			{
				OfFunction: &openai.ChatCompletionFunctionToolParam{
					Function: openai.FunctionDefinitionParam{
						Name: "Bash",
						Description: openai.String("Execute a shell command"),
						Parameters: openai.FunctionParameters{
							"type": "object",
							"properties": map[string]any{
								"command": map[string]any{
									"type": "string",
									"description": "The shell command to execute",
								},
							},

							// Bash cannot run without knowing which command the
							// model wants to execute.
							"required": []string{"command"},
						},
					},
				},
			},
		},
	}

	for {
		resp, err := client.Chat.Completions.New(ctx, params)

		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(resp.Choices) == 0 {
			panic("No choices in response")
		}

		message := resp.Choices[0].Message

		params.Messages = append(
			params.Messages,
			message.ToParam(),
		)

		// If the model did not request a tool, it has produced the final answer.
		if len(message.ToolCalls) == 0 {
			fmt.Print(message.Content)
			return
		}

		// The model can request multiple tools in one response,
		// so execute every tool call rather than only the first one.
		for _, toolCall := range message.ToolCalls {
			// Select the implementation for the tool requested by the model.
			switch toolCall.Function.Name {
			case "Read":

				// Define the expected JSON arguments for the Read tool.
				var args struct {
					FilePath string `json:"file_path"`
				}

				// Convert the model-generated JSON arguments into the Go struct.
				if err := json.Unmarshal(
					[]byte(toolCall.Function.Arguments),
					&args,
				); err != nil {
					fmt.Fprintf(
						os.Stderr,
						"invalid Read arguments: %v\n",
						err,
					)
					os.Exit(1)
				}

				// Execute the Read tool by reading the requested file.
				contents, err := os.ReadFile(args.FilePath)
				if err != nil {
					fmt.Fprintf(
						os.Stderr,
						"failed to read %s: %v\n",
						args.FilePath,
						err,
					)
					os.Exit(1)
				}

				// Do not print the file contents.
				// Instead, add them to the conversation as a tool result.
				//
				// toolCall.ID links this result to the corresponding tool call
				// in the preceding assistant message.
				params.Messages = append(
					params.Messages,
					openai.ToolMessage(
						string(contents),
						toolCall.ID,
					),
				)

			case "Write":
				var args struct {
					FilePath string `json:"file_path"`
					Content string `json:"content"`
				}

				if err := json.Unmarshal(
					[]byte(toolCall.Function.Arguments),
					&args,
				); err != nil {
					fmt.Fprintf(os.Stderr, "invalid Write arguments: %v\n", err)
					os.Exit(1)
				}

				// WriteFile creates the file if it does not exist and truncates
				// (overwrites) it if it already exists. 0644 gives the owner
				// read/write permission and other users read permission.
				if err := os.WriteFile(
					args.FilePath,
					[]byte(args.Content),
					0644,
				); err != nil {
					fmt.Fprintf(
						os.Stderr,
						"failed to write %s: %v\n",
						args.FilePath,
						err,
					)
					os.Exit(1)
				}

				// Tell the model that the operation completed. This must be a
				// tool message so the model can continue and produce its final
				// response, such as "Created the file".
				params.Messages = append(
					params.Messages,
					openai.ToolMessage("File written successfully", toolCall.ID),
				)

			case "Bash":
				// This struct describes the JSON arguments expected from the model:
				// {"command":"rm README_old.md"}
				var args struct {
					Command string `json:"command"`
				}

				// Convert the model-generated JSON into the Go struct.
				if err := json.Unmarshal(
					[]byte(toolCall.Function.Arguments),
					&args,
				); err != nil {
					fmt.Fprintf(os.Stderr, "invalid Bash arguments: %v\n", err)
					os.Exit(1)
				}

				// Run the command through the system shell.
				//
				// Using "sh -c" allows shell features such as pipes, redirection,
				// and multiple command arguments. The command inherits this
				// program's current working directory, so it operates on the
				// tester's project files rather than a temporary directory.
				cmd := exec.Command("sh", "-c", args.Command)

				// CombinedOutput captures both stdout and stderr. Shell commands
				// often report useful failure details through stderr.
				output, commandErr := cmd.CombinedOutput()
				toolOutput := string(output)

				if commandErr != nil {
					// A failed shell command is reported to the model as a tool
					// result. Do not immediately terminate the agent: the model
					// may be able to understand the error or try another command.
					if toolOutput != "" {
						toolOutput += "\n"
					}
					toolOutput += fmt.Sprintf("Command failed: %v", commandErr)
				} else if toolOutput == "" {
					// Commands such as `rm README_old.md` normally produce no
					// output when successful. Give the model an explicit result
					// so it knows the command completed.
					toolOutput = "Command executed successfully"
				}

				// Associate the command result with the exact Bash request and
				// continue the agent loop so the model can give its final answer.
				params.Messages = append(
					params.Messages,
					openai.ToolMessage(toolOutput, toolCall.ID),
				)

			default:
				// Reject tool names that this program does not implement.
				fmt.Fprintf(
					os.Stderr,
					"unsupported tool: %s\n",
					toolCall.Function.Name,
				)
				os.Exit(1)
			}
		}
	}
}
