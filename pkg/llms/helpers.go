package llms

import (
	"regexp"
	"strings"
)

func buildString(strs ...string) string {
	var sb strings.Builder

	for _, str := range strs {
		sb.WriteString("\n")
		sb.WriteString(str)
	}

	return sb.String()
}

var thinkTagPattern = regexp.MustCompile(`(?s)<think>.*?</think>\n?`)

func removeThinkingTags(response string) string {
	return thinkTagPattern.ReplaceAllString(response, "")
}
