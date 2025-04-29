package main

import (
	"codeberg.org/n30w/jasima/agent"
	"codeberg.org/n30w/jasima/chat"
)

type (
	command       func(agent.Command, ...string) commandTarget
	commandTarget func(*client) *chat.Message
)

func buildCommand(sender string) command {
	return func(
		command agent.Command,
		content ...string,
	) commandTarget {
		return func(c *client) *chat.Message {
			msg := &chat.Message{
				Sender:   sender,
				Receiver: c.name.String(),
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

func sendCommandBuilder(
	pool chan<- *chat.Message,
) func([]*client, ...commandTarget) {
	return func(clients []*client, commands ...commandTarget) {
		for _, c := range clients {
			for _, cmd := range commands {
				pool <- cmd(c)
			}
		}
	}
}
