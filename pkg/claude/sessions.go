package claude

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Kind string

const (
	KindInteractive Kind = "interactive"
	KindBackground  Kind = "bg"
)

type Status string

const (
	StatusBusy Status = "busy"
	StatusIdle Status = "idle"
	// StatusShell is a session running a shell command; the CLI counts it
	// as busy, and so does the map: only idle means idle.
	StatusShell Status = "shell"
	// StatusWaiting is a session stopped on a person: a permission prompt
	// or a question, named by the record's waitingFor.
	StatusWaiting Status = "waiting"
)

type SessionRecord struct {
	PID        int
	SessionID  string
	CWD        string
	Kind       Kind
	Status     Status
	WaitingFor string
	Name       string
	StartedAt  time.Time
	Version    string
	Entrypoint string
	JobID      string
	Path       string
}

type SkippedFile struct {
	Path string
	Err  error
}

func (s SkippedFile) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Path  string `json:"path"`
		Error string `json:"error"`
	}{s.Path, s.Err.Error()})
}

func (s *SkippedFile) UnmarshalJSON(data []byte) error {
	var raw struct {
		Path  string `json:"path"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Path, s.Err = raw.Path, errors.New(raw.Error)
	return nil
}

func (s SkippedFile) Error() string {
	return s.Path + ": " + s.Err.Error()
}

func (s SkippedFile) Unwrap() error {
	return s.Err
}

type sessionRecordJSON struct {
	PID        int    `json:"pid"`
	SessionID  string `json:"sessionId"`
	CWD        string `json:"cwd"`
	Kind       Kind   `json:"kind"`
	Status     Status `json:"status"`
	WaitingFor string `json:"waitingFor"`
	Name       string `json:"name"`
	StartedAt  int64  `json:"startedAt"`
	Version    string `json:"version"`
	Entrypoint string `json:"entrypoint"`
	JobID      string `json:"jobId"`
}

func ReadSessionRecords(dir string) ([]SessionRecord, []SkippedFile, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, nil, err
	}
	var records []SessionRecord
	var skipped []SkippedFile
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			skipped = append(skipped, SkippedFile{Path: path, Err: err})
			continue
		}
		var raw sessionRecordJSON
		if err := json.Unmarshal(data, &raw); err != nil {
			skipped = append(skipped, SkippedFile{Path: path, Err: err})
			continue
		}
		records = append(records, SessionRecord{
			PID:        raw.PID,
			SessionID:  raw.SessionID,
			CWD:        raw.CWD,
			Kind:       raw.Kind,
			Status:     raw.Status,
			WaitingFor: raw.WaitingFor,
			Name:       raw.Name,
			StartedAt:  time.UnixMilli(raw.StartedAt).UTC(),
			Version:    raw.Version,
			Entrypoint: raw.Entrypoint,
			JobID:      raw.JobID,
			Path:       path,
		})
	}
	return records, skipped, nil
}
