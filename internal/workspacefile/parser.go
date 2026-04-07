package workspacefile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ParseFile(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	doc := &Document{Source: path}
	scanner := bufio.NewScanner(f)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		fields := splitFields(trimmed)
		if len(fields) == 0 {
			continue
		}

		doc.Instructions = append(doc.Instructions, Instruction{
			Keyword: strings.ToUpper(fields[0]),
			Args:    fields[1:],
			Line:    lineNo,
			Raw:     trimmed,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan file: %w", err)
	}

	return doc, nil
}

func splitFields(input string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false

	for _, r := range input {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' || r == '\t':
			if inQuote {
				current.WriteRune(r)
				continue
			}
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
