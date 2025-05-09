package network

import (
	"codeberg.org/n30w/jasima/pkg/agent"
	"codeberg.org/n30w/jasima/pkg/chat"
)

type (
	CommandForAgent func(agent.Command, ...string) MessageFor
	MessageFor      func(client *GRPCClient) *chat.Message
)

func BuildCommand(sender string) CommandForAgent {
	return func(
		command agent.Command,
		content ...string,
	) MessageFor {
		return func(c *GRPCClient) *chat.Message {
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
	pool chan<- *chat.Message,
) func([]*GRPCClient, ...MessageFor) {
	return func(clients []*GRPCClient, commands ...MessageFor) {
		for _, c := range clients {
			for _, cmd := range commands {
				pool <- cmd(c)
			}
		}
	}
}
