package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	USER   string = "\\u001b[94mYou\\u001b[0m: "
	CLAUDE string = "\\u001b[93mClaude\\u001b[0m: %s\n"
)

type Agent struct {
	client    *anthropic.Client
	UserInput io.Reader
	Output    io.Writer
}

type option func(*Agent) error

// runInference sends messages to the Claude API and returns the response
func (a *Agent) runInference(ctx context.Context, conversation []anthropic.MessageParam) (*anthropic.Message, error) {

	messageParams := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_8,
		MaxTokens: int64(1042),
		Messages:  conversation,
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
	fmt.Fprint(a.Output, USER)

	conversation := []anthropic.MessageParam{}
	scanner := bufio.NewScanner(a.UserInput)

	for scanner.Scan() {
		fmt.Fprint(a.Output, USER)

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

func NewAgent(opts ...option) (*Agent, error) {
	a := &Agent{
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
	client := anthropic.NewClient()

	agent, err := NewAgent(
		WithClient(&client),
	)

	err = agent.Run(context.TODO())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}
}
