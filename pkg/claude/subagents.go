package claude

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const agentFilePrefix = "agent-"

type Subagent struct {
	Transcript
	AgentID    string
	AgentType  string
	ToolUseID  string
	SpawnDepth int
}

type subagentMetaJSON struct {
	AgentType  string `json:"agentType"`
	ToolUseID  string `json:"toolUseId"`
	SpawnDepth int    `json:"spawnDepth"`
}

func SubagentsDir(transcriptPath string) string {
	return filepath.Join(strings.TrimSuffix(transcriptPath, ".jsonl"), "subagents")
}

func ReadSubagents(transcriptPath string) ([]Subagent, []SkippedFile, error) {
	paths, err := SubagentPaths(transcriptPath)
	if err != nil {
		return nil, nil, err
	}
	var subagents []Subagent
	var skipped []SkippedFile
	for _, path := range paths {
		subagent, metaSkipped, err := ReadSubagent(path)
		if err != nil {
			skipped = append(skipped, SkippedFile{Path: path, Err: err})
			continue
		}
		skipped = append(skipped, metaSkipped...)
		subagents = append(subagents, subagent)
	}
	return subagents, skipped, nil
}

func SubagentPaths(transcriptPath string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(SubagentsDir(transcriptPath), agentFilePrefix+"*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func ReadSubagent(path string) (Subagent, []SkippedFile, error) {
	transcript, err := readTranscriptFile(path, true)
	if err != nil {
		return Subagent{}, nil, err
	}
	subagent, skipped := subagentOf(path, transcript)
	return subagent, skipped, nil
}

// SubagentReader reads a subagent's transcript as it grows, as
// TranscriptReader does, and its meta file whole each time. Item 141.
type SubagentReader struct {
	transcript *TranscriptReader
}

func NewSubagentReader(path string) *SubagentReader {
	return &SubagentReader{transcript: &TranscriptReader{path: path, sidechainIsMain: true}}
}

func (r *SubagentReader) Read() (Subagent, []SkippedFile, error) {
	transcript, err := r.transcript.Read()
	if err != nil {
		return Subagent{}, nil, err
	}
	subagent, skipped := subagentOf(r.transcript.path, transcript)
	return subagent, skipped, nil
}

func subagentOf(path string, transcript Transcript) (Subagent, []SkippedFile) {
	stem := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	subagent := Subagent{Transcript: transcript, AgentID: strings.TrimPrefix(stem, agentFilePrefix)}
	var skipped []SkippedFile
	metaPath := strings.TrimSuffix(path, ".jsonl") + ".meta.json"
	if meta, err := readSubagentMeta(metaPath); err != nil {
		skipped = append(skipped, SkippedFile{Path: metaPath, Err: err})
	} else if meta != nil {
		subagent.AgentType = meta.AgentType
		subagent.ToolUseID = meta.ToolUseID
		subagent.SpawnDepth = meta.SpawnDepth
	}
	return subagent, skipped
}

func readSubagentMeta(path string) (*subagentMetaJSON, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var meta subagentMetaJSON
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
