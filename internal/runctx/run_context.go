package runctx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"
)

type Context struct {
	RunID     string
	InputPath string
	Dir       string
	CreatedAt time.Time
}

func New(inputPath string, createdAt time.Time) (Context, error) {
	runID, err := NewRunID(createdAt)
	if err != nil {
		return Context{}, err
	}

	return Context{
		RunID:     runID,
		InputPath: inputPath,
		Dir:       filepath.Join(".byom-video", "runs", runID),
		CreatedAt: createdAt,
	}, nil
}

func NewRunID(t time.Time) (string, error) {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("create run id: %w", err)
	}
	return fmt.Sprintf("%s-%s", t.UTC().Format("20060102T150405Z"), hex.EncodeToString(suffix[:])), nil
}
