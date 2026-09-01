package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

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

		// You can use print statements as follows for debugging, they'll be visible when running tests.
		fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

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
			// Read is currently the only tool provided to the model.
			if toolCall.Function.Name != "Read" {
				fmt.Fprintf(
					os.Stderr,
					"unsupported tool: %s\n",
					toolCall.Function.Name,
				)
				os.Exit(1)
			}

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
		}

	}

}
