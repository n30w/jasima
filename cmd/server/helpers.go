package main

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/n30w/jasima/agent"
	"codeberg.org/n30w/jasima/chat"
)

func makePortString(p string) string {
	return ":" + p
}

func newLangSpecification(p string) (chat.LayerMessageSet, error) {
	ls := make(chat.LayerMessageSet)

	b, err := os.ReadFile(filepath.Join(p, "dictionary.md"))
	if err != nil {
		return nil, err
	}

	ls[chat.DictionaryLayer] = chat.Content(b)

	b, err = os.ReadFile(filepath.Join(p, "grammar.md"))
	if err != nil {
		return nil, err
	}

	ls[chat.GrammarLayer] = chat.Content(b)

	b, err = os.ReadFile(filepath.Join(p, "logography.md"))
	if err != nil {
		return nil, err
	}

	ls[chat.LogographyLayer] = chat.Content(b)

	b, err = os.ReadFile(filepath.Join(p, "phonetics.md"))
	if err != nil {
		return nil, err
	}

	ls[chat.PhoneticsLayer] = chat.Content(b)

	return ls, nil
}

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

func memoryToString(m MemoryService) string {
	return fmt.Sprintf(
		"=== BEGIN CHAT LOG ===\n%s\n=== END CHAT LOG ===",
		m.String(),
	)
}
