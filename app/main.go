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
	resp, err := client.Chat.Completions.New(context.Background(),
		openai.ChatCompletionNewParams{
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
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(resp.Choices) == 0 {
		panic("No choices in response")
	}

	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	// TODO: Uncomment the line below to pass the first stage
	// fmt.Print(resp.Choices[0].Message.Content)

	message := resp.Choices[0].Message

	if len(message.ToolCalls) == 0 {
		fmt.Print(message.Content)
		return
	}

	// This stage requires executing only the first tool call.
	toolCall := message.ToolCalls[0]

	if toolCall.Function.Name != "Read" {
		// what does this line even mean?
		fmt.Fprintf(os.Stderr, "unsupported tool: %s\n", toolCall.Function.Name)
		os.Exit(1)
	}

	var args struct {
		FilePath string `json:"file_path"`
	}

	if err := json.Unmarshal(
		[]byte(toolCall.Function.Arguments),
		&args,
	); err != nil {
		fmt.Fprintf(os.Stderr, "invalid Read arguments: %v\n", err)
		os.Exit(1)
	}

	// here we are able to get the filePath properly, now we can read it and send the contents to our LLM to summarize it

	contents, err := os.ReadFile(args.FilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read %s: %v\n", args.FilePath, err)
		os.Exit(1)
	}

	// diff in print, Print, Fprintf?
	fmt.Print(string(contents))
}
