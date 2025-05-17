package main

import (
	"context"
	"time"

	"codeberg.org/n30w/jasima/pkg/agent"
	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"

	"github.com/pkg/errors"
)

func (c *client) Teardown() error {
	c.logger.Debug("Beginning teardown...")

	err := c.mc.Close()
	if err != nil {
		return errors.Wrap(err, "teardown failure")
	}

	return nil
}

func (c *client) DispatchToLLM(ctx context.Context) {
	select {
	case <-ctx.Done():
		c.logger.Warn("dispatch context cancelled")
		return
	default:
		a, err := c.stm.Retrieve(ctx, c.Name, 0)
		if err != nil {
			c.channels.errs <- err
			return
		}

		time.Sleep(time.Second * c.sleepDuration)

		c.logger.Debug("Sending message to LLM now!")

		t := utils.Timer(time.Now())

		// Make the request

		result, err := c.llm.Request(ctx, a, nil)
		switch {
		case errors.Is(err, context.Canceled):
			c.logger.Warn("llm request context cancelled")
			return
		case err != nil:
			c.channels.errs <- errors.Wrap(err, "request to llm failed")
			return
		}

		c.logger.Debugf("LLM responded in %s", t().Truncate(1*time.Millisecond))

		res := chat.Content(result)

		// Save the LLM's response to memory.

		newMsg := c.NewMessageTo(c.Peers[0], res)

		err = c.stm.Save(ctx, newMsg)
		if err != nil {
			c.channels.errs <- err
			return
		}

		time.Sleep(time.Second * c.sleepDuration)

		c.channels.responses <- newMsg
	}
}

// SendMessages listens on the responses channel for messages. When a message
// is received, it sends the message to the intended recipients.
func (c *client) SendMessages() {
	for res := range c.channels.responses {

		err := c.sendMessage(res)
		if err != nil {
			c.channels.errs <- err
			return
		}

		c.logger.Debug("📧 message sent to peers successfully!")
	}
}

// ReceiveMessages listens for messages incoming from the server.
func (c *client) ReceiveMessages() {
	err := c.mc.Receive()
	if err != nil {
		c.channels.errs <- err
	}
}

func (c *client) Router() {
	var (
		printConsoleData = func(ctx context.Context, pbMsg *chat.Message) error {
			msg := memory.NewChatMessage(
				pbMsg.Sender, pbMsg.Receiver,
				pbMsg.Content, pbMsg.Layer, pbMsg.Command,
			)

			if msg.Sender != "SERVER" {
				c.logger.Debugf("Message received from %s", msg.Sender)
			}

			// Intercept commands from the server.

			if msg.Command != agent.NoCommand {
				c.logger.Debugf("Received %s", msg.Command)
			}

			return nil
		}

		prevCancel context.CancelFunc

		// id is the context id. It increments everytime a new message is
		// received.
		id int

		messageRouter = func(ctx context.Context, pbMsg *chat.Message) error {
			msgCtx, cancel := context.WithCancel(ctx)
			msg := memory.NewChatMessage(
				pbMsg.Sender, pbMsg.Receiver,
				pbMsg.Content, pbMsg.Layer, pbMsg.Command,
			)

			err := c.action(msgCtx, prevCancel, id, msg)
			if err != nil {
				cancel()
				return err
			}

			prevCancel = cancel

			// go func(parentCtx context.Context, ctxId int) {
			// 	ctx, cancel := context.WithCancelCause(parentCtx)
			// 	defer cancel(context.Cause(parentCtx))
			// 	<-ctx.Done()
			// 	err := context.Cause(ctx)
			// 	if err != nil {
			// 		c.logger.Warnf("Cancelled context %d because %s", ctxId, err)
			// 	}
			// }(msgCtx, id)

			id++

			return nil
		}

		saveMessage = func(ctx context.Context, pbMsg *chat.Message) error {
			msg := memory.NewChatMessage(
				pbMsg.Sender, pbMsg.Receiver,
				pbMsg.Content, pbMsg.Layer, pbMsg.Command,
			)

			if msg.Command == agent.NoCommand && msg.Text != "" {
				err := c.stm.Save(ctx, c.NewMessageFrom(msg.Sender, msg.Text))
				if err != nil {
					return err
				}
			}

			return nil
		}
	)

	routeMessage := chat.BuildRouter[*chat.Message](
		c.channels.inbound,
		printConsoleData,
		saveMessage,
		messageRouter,
	)

	go routeMessage(c.channels.errs)
}

func (c *client) Run(ctx context.Context) {
	c.Router()

	go c.SendMessages()

	go c.ReceiveMessages()

	err := c.initConnection()
	if err != nil {
		c.channels.errs <- err
		return
	}
}
