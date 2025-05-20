package network

import (
	"context"
	"sync"

	"codeberg.org/n30w/jasima/pkg/agent"
	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/utils"
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
		for _, c := range clients {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case <-ctx.Done():
					return
				default:
					for _, cmd := range commands {
						_ = utils.SendWithContext(ctx, pool, cmd(c))
					}
				}
			}()
			wg.Wait()
		}
	}
}
