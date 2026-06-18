package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
)

type Agent struct {
	client         *anthropic.Client
	getUserMessage func() (string, bool)
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
	conversation := []anthropic.MessageParam{}

	fmt.Println("Chat with Claude (use 'ctrl-c' to quit)")

	for {
		fmt.Print("\\u001b[94mYou\\u001b[0m: ")

		// Collect user input
		userInput, ok := a.getUserMessage()

		// If there's no user input, there's no need to continue the loop.
		if !ok {
			break
		}

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
				fmt.Printf("\\u001b[93mClaude\\u001b[0m: %s\n", content.Text)
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

func WithGetUserMessage(c func() (string, bool)) option {
	return func(a *Agent) error {
		if c == nil {
			return errors.New("nil is not a valid function")
		}
		a.getUserMessage = c
		return nil
	}
}

func NewAgent(opts ...option) (*Agent, error) {
	a := &Agent{}

	for _, opt := range opts {
		err := opt(a)
		if err != nil {
			return nil, err
		}
	}

	return a, nil
}

func OldAgent(client *anthropic.Client, getUserMessage func() (string, bool)) *Agent {
	return &Agent{
		client:         client,
		getUserMessage: getUserMessage,
	}
}

func main() {
	client := anthropic.NewClient()

	scanner := bufio.NewScanner(os.Stdin)

	// bool is treated as an indicator whether the operation completed successfully.
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	agent, err := NewAgent(
		WithClient(&client),
		WithGetUserMessage(getUserMessage),
	)

	err = agent.Run(context.TODO())
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}
