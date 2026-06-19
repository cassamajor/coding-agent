package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	USER   string = "\x1b[94mYou\x1b[0m: "        // Colorize the text "You"
	CLAUDE string = "\x1b[93mClaude\x1b[0m: %s\n" // Colorize the text "Claude"
)

type Agent struct {
	client    *anthropic.Client
	UserInput io.Reader
	Output    io.Writer
	Tools     []ToolDefinition
}

type option func(*Agent) error

type ToolDefinition struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	InputSchema anthropic.ToolInputSchemaParam `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

// runInference sends messages to the Claude API and returns the response
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
// 1. Take user input, and add it to the conversation slice.
// 2. Send the conversation to Claude.
// 3. Claude responds, which we also add to the conversation slice.
// 4. Repeat
func (a *Agent) Run(ctx context.Context) error {
	fmt.Fprintln(a.Output, "Chat with Claude (use 'ctrl-c' to quit)")

	conversation := []anthropic.MessageParam{}
	scanner := bufio.NewScanner(a.UserInput)

	for {
		fmt.Fprint(a.Output, USER)

		if !scanner.Scan() {
			break
		}

		userInput := scanner.Text()

		// Store user input
		userMessage := anthropic.NewUserMessage(
			anthropic.NewTextBlock(userInput),
		)

		conversation = append(conversation, userMessage)

		// Send user input to Anthropic API and receive a response
		message, err := a.runInference(ctx, conversation)
		if err != nil {
			return err
		}

		// Store the agent response
		conversation = append(conversation, message.ToParam())

		// Share the agent response with the user
		for _, content := range message.Content {
			switch content.Type {
			case "text":
				fmt.Fprintf(a.Output, CLAUDE, content.Text)
			}
		}
	}

	return nil
}

func WithClient(c *anthropic.Client) option {
	return func(a *Agent) error {
		if c == nil {
			return errors.New("nil is not a valid client")
		}
		a.client = c
		return nil
	}
}

func WithInput(r io.Reader) option {
	return func(a *Agent) error {
		if r == nil {
			return errors.New("nil is not a valid reader")
		}
		a.UserInput = r
		return nil
	}
}

func WithOutput(w io.Writer) option {
	return func(a *Agent) error {
		if w == nil {
			return errors.New("nil is not a valid writer")
		}
		a.Output = w
		return nil
	}
}

func WithTools(td []ToolDefinition) option {
	return func(a *Agent) error {
		if td == nil {
			return errors.New("nil is not a valid tool definition")
		}
		a.Tools = td
		return nil
	}
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

func main() {
	agent, err := NewAgent()

	err = agent.Run(context.TODO())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
	}
}
