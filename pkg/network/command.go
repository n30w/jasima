package network

import (
	"context"

	"codeberg.org/n30w/jasima/pkg/agent"
	"codeberg.org/n30w/jasima/pkg/chat"
)

type (
	CommandForAgent func(agent.Command, ...string) MessageFor
	MessageFor      func(client *ChatClient) *chat.Message
	CommandsSender  func([]*ChatClient, ...MessageFor)
)

func BuildCommand(sender string) CommandForAgent {
	return func(
		command agent.Command,
		content ...string,
	) MessageFor {
		return func(c *ChatClient) *chat.Message {
			msg := &chat.Message{
				Sender:   sender,
				Receiver: c.Name.String(),
				Command:  command.Int32(),
				Layer:    c.layer.Int32(),
				Content:  "",
			}

			if len(content) > 0 {
				msg.Content = content[0]
			}

			return msg
		}
	}
}

func SendCommandBuilder(
	ctx context.Context,
	pool chan<- *chat.Message,
) func(
	[]*ChatClient,
	...MessageFor,
) {
	return func(clients []*ChatClient, commands ...MessageFor) {
		sCtx, sCancel := context.WithCancel(ctx)
		defer sCancel()
		for _, c := range clients {
			for _, cmd := range commands {
				select {
				case <-sCtx.Done():
					return
				case pool <- cmd(c):
				}
			}
		}
	}
}
