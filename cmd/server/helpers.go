package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"codeberg.org/n30w/jasima/pkg/agent"
	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/network"

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

func loadJsonFile[T any](p string) ([]T, error) {
	var a []T

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	b, _ := io.ReadAll(f)

	err = json.Unmarshal(b, &a)
	if err != nil {
		return nil, err
	}

	return a, nil
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

func findUsedWords(
	dict memory.DictionaryGeneration,
	text string,
) chat.AgentDictionaryWordsDetectionResponse {
	res := chat.AgentDictionaryWordsDetectionResponse{
		Words: make([]string, 0),
	}

	wordSet := make(map[string]struct{})
	textLower := strings.ToLower(text)

	for _, v := range dict {
		wordLower := strings.ToLower(v.Word)
		pattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(wordLower))
		re := regexp.MustCompile(pattern)

		if re.MatchString(textLower) {
			if _, exists := wordSet[wordLower]; !exists {
				res.Words = append(res.Words, v.Word)
				wordSet[wordLower] = struct{}{}
			}
		}
	}

	return res
}

func (s *ConlangServer) getExtractedWordsFromText(
	cmd network.CommandForAgent,
	newGeneration memory.Generation,
	m memory.Message,
) (chat.AgentDictionaryWordsDetectionResponse, error) {
	var dictionaryWords chat.AgentDictionaryWordsDetectionResponse

	sysAgentDictExtractor, err := s.gs.GetClientByName("SYSTEM_AGENT_C")
	if err != nil {
		return dictionaryWords, errors.Wrap(
			err,
			"failed to retrieve client by name",
		)
	}

	var sb2 strings.Builder

	sb2.WriteString(newGeneration.Dictionary.String())

	s.gs.Channel.ToClients <- cmd(agent.Latch)(sysAgentDictExtractor)
	s.gs.Channel.ToClients <- cmd(
		agent.AppendInstructions,
		sb2.String(),
	)(sysAgentDictExtractor)
	s.gs.Channel.ToClients <- cmd(agent.Unlatch)(sysAgentDictExtractor)
	s.gs.Channel.ToClients <- cmd(
		agent.RequestDictionaryWordDetection,
		m.Text.String(),
	)(sysAgentDictExtractor)

	words := <-s.gs.Channel.ToServer

	err = json.Unmarshal([]byte(words.Text), &dictionaryWords)
	if err != nil {
		return dictionaryWords, errors.Wrap(
			err,
			"failed to unmarshal dictionary words",
		)
	}

	s.gs.Channel.ToClients <- cmd(agent.Latch)(sysAgentDictExtractor)
	s.gs.Channel.ToClients <- cmd(agent.ClearMemory)(sysAgentDictExtractor)
	s.gs.Channel.ToClients <- cmd(agent.ResetInstructions)(sysAgentDictExtractor)

	return dictionaryWords, nil
}
