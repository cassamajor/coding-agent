package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/invopop/jsonschema"
)

const (
	USER   string = "\x1b[94mYou\x1b[0m: "          // Colorize the text "You"
	CLAUDE string = "\x1b[93mClaude\x1b[0m: %s\n"   // Colorize the text "Claude"
	TOOL   string = "\x1b[92mtool\x1b[0m: %s(%s)\n" // Colorize the text "tool"
)

// Description should follow best practices: a brief explanation, specifiy the circumstances the tool should be used, and the circumstances that it should not be used.
type ToolDefinition struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	InputSchema anthropic.ToolInputSchemaParam `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

type Agent struct {
	client    *anthropic.Client
	UserInput io.Reader
	Output    io.Writer
	Tools     []ToolDefinition
}

func (a *Agent) executeTool(content *anthropic.ContentBlockUnion) anthropic.ContentBlockParamUnion {
	var toolDef ToolDefinition
	var found bool

	for _, tool := range a.Tools {
		if tool.Name == content.Name {
			toolDef = tool
			found = true
			break
		}
	}
	if !found {
		return anthropic.NewToolResultBlock(content.ID, "tool not found", true)
	}

	fmt.Fprintf(a.Output, TOOL, content.Name, content.Input)

	// Call the function assigned to the tool definition.
	response, err := toolDef.Function(content.Input)

	// The tool function returned an error
	if err != nil {
		return anthropic.NewToolResultBlock(content.ID, err.Error(), true)
	}

	// Return the content produced by the tool function
	return anthropic.NewToolResultBlock(content.ID, response, false)
}

// runInference sends messages to the Claude API and returns the response. It also specifies which tools are available to the agent.
func (a *Agent) runInference(ctx context.Context, conversation []anthropic.MessageParam) (*anthropic.Message, error) {
	anthropicTools := []anthropic.ToolUnionParam{}

	for _, t := range a.Tools {
		oftool := &anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: t.InputSchema,
		}

		tool := anthropic.ToolUnionParam{OfTool: oftool}

		anthropicTools = append(anthropicTools, tool)
	}

	messageParams := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_8,
		MaxTokens: int64(1042),
		Messages:  conversation,
		Tools:     anthropicTools,
	}

	message, err := a.client.Messages.New(ctx, messageParams)
	return message, err
}

// Run let's us talk to Claude in a loop.
//  1. Take user input, and add it to the conversation slice.
//  2. Send the conversation to Claude.
//  3. Claude responds, which we also add to the conversation slice.
//  4. Repeat
//  5. Tool usage example: Claude's response + the tool call are returned in a single message. Then, readUserInput is set to false,
//     and the tool result is sent to Claude, and Claude responds. readUserInput is then set to true, which then allows us to continue
//     the conversation.
func (a *Agent) Run(ctx context.Context) error {
	fmt.Fprintln(a.Output, "Chat with Claude (use 'ctrl-c' to quit)")

	conversation := []anthropic.MessageParam{}

	// Collect user input
	scanner := bufio.NewScanner(a.UserInput)

	readUserInput := true
	for {
		if readUserInput {

			fmt.Fprint(a.Output, USER)

			// If there's no user input, there's no need to continue the loop.
			if !scanner.Scan() {
				break
			}

			userInput := scanner.Text()

			// Store user input
			userMessage := anthropic.NewUserMessage(
				anthropic.NewTextBlock(userInput),
			)

			conversation = append(conversation, userMessage)
		}

		// Send user input to Anthropic API and receive a response
		message, err := a.runInference(ctx, conversation)
		if err != nil {
			return err
		}

		// Store the agent response
		conversation = append(conversation, message.ToParam())

		toolResults := []anthropic.ContentBlockParamUnion{}

		// Share the agent response with the user
		for _, content := range message.Content {
			switch content.Type {
			case "text":
				fmt.Fprintf(a.Output, CLAUDE, content.Text)
			case "tool_use":
				result := a.executeTool(&content)
				toolResults = append(toolResults, result)
			}
		}
		if len(toolResults) == 0 {
			readUserInput = true
			continue
		}
		readUserInput = false
		conversation = append(conversation, anthropic.NewUserMessage(toolResults...))
	}

	return nil
}

func NewAgent(opts ...option) (*Agent, error) {
	client := anthropic.NewClient()

	a := &Agent{
		client:    &client,
		UserInput: os.Stdin,
		Output:    os.Stdout,
	}

	for _, opt := range opts {
		err := opt(a)
		if err != nil {
			return nil, err
		}
	}

	return a, nil
}

func GenerateSchema[T any]() anthropic.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}

	var v T

	schema := reflector.Reflect(v)

	return anthropic.ToolInputSchemaParam{
		Properties: schema.Properties,
	}
}

func main() {
	tools := []ToolDefinition{ReadFileDefinition, ListFilesDefinition, EditFileDefinition}

	agent, err := NewAgent(
		WithTools(tools),
	)

	err = agent.Run(context.TODO())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
	}
}
