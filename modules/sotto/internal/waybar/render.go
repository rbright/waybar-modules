package waybar

import (
	"encoding/json"
	"fmt"
)

type Output struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
	Class   string `json:"class"`
}

func Encode(output Output) ([]byte, error) {
	payload, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("marshal waybar output: %w", err)
	}
	return payload, nil
}
