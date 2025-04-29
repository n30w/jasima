package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/n30w/jasima/agent"
	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/memory"
	"github.com/pkg/errors"
)

func makePortString(p string) string {
	return ":" + p
}

func loadSVGsFromDirectory(dirPath string) (
	memory.LogographyGeneration,
	error,
) {
	svgs := make(memory.LogographyGeneration)

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		f := file.Name()
		ext := filepath.Ext(f)
		name := f[0 : len(f)-len(ext)]

		if ext == ".svg" {
			fullPath := filepath.Join(dirPath, file.Name())

			data, err := os.ReadFile(fullPath)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to read file %s: %w",
					file.Name(),
					err,
				)
			}

			svgs[name] = string(data)
		}
	}

	return svgs, nil
}

func newLangSpecification(p string) (memory.SpecificationGeneration, error) {
	ls := make(memory.SpecificationGeneration)

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

func transcriptToString(transcript []memory.Message) string {
	var sb strings.Builder

	sb.WriteString("=== BEGIN CHAT LOG ===\n")

	for _, m := range transcript {
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.Sender, m.Text))
	}

	sb.WriteString("=== END CHAT LOG ===\n")

	return sb.String()
}

func saveToJson(data any, fileName string) error {
	d, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to save JSON")
	}

	err = os.WriteFile(fileName, d, 0o644)
	if err != nil {
		return errors.Wrap(err, "failed to write file")
	}

	return nil
}
