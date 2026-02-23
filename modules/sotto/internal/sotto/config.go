package sotto

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AudioSelection struct {
	Input    string
	Fallback string
}

func ReadAudioSelection(path string) (AudioSelection, error) {
	root, err := loadRoot(path)
	if err != nil {
		return AudioSelection{}, err
	}

	audioMap, _ := ensureObject(root, "audio")
	selection := AudioSelection{
		Input:    "default",
		Fallback: "default",
	}
	if rawInput, ok := audioMap["input"].(string); ok {
		if trimmed := strings.TrimSpace(rawInput); trimmed != "" {
			selection.Input = trimmed
		}
	}
	if rawFallback, ok := audioMap["fallback"].(string); ok {
		if trimmed := strings.TrimSpace(rawFallback); trimmed != "" {
			selection.Fallback = trimmed
		}
	}

	return selection, nil
}

func SetAudioInput(path string, input string) error {
	trimmedInput := strings.TrimSpace(input)
	if trimmedInput == "" {
		return fmt.Errorf("audio input cannot be empty")
	}

	root, err := loadRoot(path)
	if err != nil {
		return err
	}

	audioMap, _ := ensureObject(root, "audio")
	audioMap["input"] = trimmedInput
	if _, ok := audioMap["fallback"]; !ok {
		audioMap["fallback"] = "default"
	}
	root["audio"] = audioMap

	payload, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal updated config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	return writeFileAtomically(path, append(payload, '\n'))
}

func loadRoot(path string) (map[string]any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read sotto config %q: %w", path, err)
	}

	normalized, err := normalizeJSONC(string(content))
	if err != nil {
		return nil, fmt.Errorf("normalize JSONC config %q: %w", path, err)
	}

	decoder := json.NewDecoder(strings.NewReader(normalized))
	decoder.UseNumber()

	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("decode config %q: %w", path, err)
	}
	if root == nil {
		root = map[string]any{}
	}

	return root, nil
}

func ensureObject(root map[string]any, key string) (map[string]any, bool) {
	if root == nil {
		root = map[string]any{}
	}
	if existing, ok := root[key]; ok {
		if objectMap, ok := existing.(map[string]any); ok {
			return objectMap, true
		}
	}
	return map[string]any{}, false
}

func writeFileAtomically(path string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace config file: %w", err)
	}
	return nil
}

func normalizeJSONC(content string) (string, error) {
	withoutComments, err := stripJSONCComments(content)
	if err != nil {
		return "", err
	}
	return stripJSONCTrailingCommas(withoutComments), nil
}

func stripJSONCComments(content string) (string, error) {
	var out strings.Builder
	out.Grow(len(content))

	inString := false
	escape := false
	lineComment := false
	blockComment := false

	for i := 0; i < len(content); i++ {
		ch := content[i]

		if lineComment {
			if ch == '\n' || ch == '\r' {
				lineComment = false
				out.WriteByte(ch)
				continue
			}
			out.WriteByte(' ')
			continue
		}

		if blockComment {
			if ch == '*' && i+1 < len(content) && content[i+1] == '/' {
				blockComment = false
				out.WriteString("  ")
				i++
				continue
			}
			if ch == '\n' || ch == '\r' || ch == '\t' {
				out.WriteByte(ch)
			} else {
				out.WriteByte(' ')
			}
			continue
		}

		if inString {
			out.WriteByte(ch)
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}

		if ch == '/' && i+1 < len(content) {
			next := content[i+1]
			if next == '/' {
				lineComment = true
				out.WriteString("  ")
				i++
				continue
			}
			if next == '*' {
				blockComment = true
				out.WriteString("  ")
				i++
				continue
			}
		}

		out.WriteByte(ch)
	}

	if blockComment {
		return "", fmt.Errorf("unterminated block comment in JSONC")
	}

	return out.String(), nil
}

func stripJSONCTrailingCommas(content string) string {
	var out strings.Builder
	out.Grow(len(content))

	inString := false
	escape := false

	for i := 0; i < len(content); i++ {
		ch := content[i]

		if inString {
			out.WriteByte(ch)
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}

		if ch == ',' {
			j := i + 1
			for j < len(content) && isJSONWhitespace(content[j]) {
				j++
			}
			if j < len(content) && (content[j] == '}' || content[j] == ']') {
				continue
			}
		}

		out.WriteByte(ch)
	}

	return out.String()
}

func isJSONWhitespace(ch byte) bool {
	switch ch {
	case ' ', '\n', '\r', '\t':
		return true
	default:
		return false
	}
}
