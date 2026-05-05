package events

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Log struct {
	file *os.File
	enc  *json.Encoder
}

type Event struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Details any       `json:"details,omitempty"`
}

func Open(path string) (*Log, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open event log: %w", err)
	}
	return &Log{file: file, enc: json.NewEncoder(file)}, nil
}

func (l *Log) Write(eventType string, details any) error {
	if err := l.enc.Encode(Event{
		Time:    time.Now().UTC(),
		Type:    eventType,
		Details: details,
	}); err != nil {
		return fmt.Errorf("write event %s: %w", eventType, err)
	}
	return nil
}

func (l *Log) Close() error {
	return l.file.Close()
}
