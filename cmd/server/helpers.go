package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/memory"

	"github.com/pkg/errors"
)

func newTranscriptGeneration() memory.TranscriptGeneration {
	t := make(memory.TranscriptGeneration)

	for i := range chat.LogographyLayer + 1 {
		t[i] = make([]memory.Message, 0)
	}

	return t
}

func loadLogographySvgsFromFile(dirPath string) (
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

func loadSpecificationsFromFile(p string) (
	memory.SpecificationGeneration,
	error,
) {
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

	ls[chat.SystemLayer] = chat.Content("")

	return ls, nil
}

func loadDictionaryFromFile(p string) (memory.DictionaryGeneration, error) {
	dict := make(memory.DictionaryGeneration)

	data, err := os.ReadFile(p)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load dictionary file %s", p)
	}

	var entries []memory.DictionaryEntry

	err = json.Unmarshal(data, &entries)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load dictionary file %s", p)
	}

	for _, entry := range entries {
		dict[entry.Word] = entry
	}

	return dict, nil
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
