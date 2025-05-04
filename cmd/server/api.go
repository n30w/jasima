package main

import (
	"context"

	"github.com/pkg/errors"

	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/memory"
)

func (s *ConlangServer) messageToSystemAgent(
	name chat.Name,
	msg string,
) *chat.Message {
	return chat.NewPbMessage(
		s.gs.Name,
		name,
		chat.Content(msg),
		chat.SystemLayer,
	)
}

func saveMessageTo(
	ctx context.Context,
	mem MemoryService,
	msg memory.Message,
) error {
	msg.Role = memory.UserRole
	err := mem.Save(ctx, msg)
	if err != nil {
		return errors.Wrap(err, "failed to save message")
	}

	return nil
}
