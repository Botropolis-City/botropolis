package stats

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

// IngestPaths is Ingest for the transcripts named alone, main and
// subagent ones under projectsDir, for a caller that knows which changed
// since the last ingest: it reads nothing about the rest, and looks for
// sessions whose transcripts are gone only when listed says the set of
// transcripts may have changed (item 205).
func (s *Store) IngestPaths(projectsDir string, paths []string, listed bool, now time.Time, yield func()) error {
	rows, err := s.transcriptsOf(paths)
	if err != nil {
		return err
	}
	known, err := s.left(rows, paths)
	if err != nil {
		return err
	}
	s.done.Store(0)
	s.known.Store(known)
	if yield == nil {
		yield = func() {}
	}
	step := func() error {
		yield()
		return nil
	}
	for _, path := range paths {
		session, thread, ok := transcriptOf(projectsDir, path)
		if !ok {
			continue
		}
		yield()
		if err := s.ingestFile(rows, path, session, thread, step); err != nil {
			return err
		}
	}
	if !listed {
		return nil
	}
	s.index.Refresh()
	return s.markGone(s.index.Transcripts(projectsDir), now)
}

// transcriptOf is the session and thread a path under projectsDir holds:
// a main transcript is <project>/<session>.jsonl, and a subagent's
// <project>/<session>/subagents/agent-<thread>.jsonl.
func transcriptOf(projectsDir, path string) (session, thread string, ok bool) {
	rel, err := filepath.Rel(projectsDir, path)
	if err != nil || !strings.HasSuffix(rel, ".jsonl") {
		return "", "", false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	switch {
	case len(parts) == 2:
		return strings.TrimSuffix(parts[1], ".jsonl"), mainThread, true
	case len(parts) == 4 && parts[2] == "subagents" && strings.HasPrefix(parts[3], agentPrefix):
		return parts[1], strings.TrimSuffix(strings.TrimPrefix(parts[3], agentPrefix), ".jsonl"), true
	}
	return "", "", false
}

// transcriptsOf is the rows of the transcripts named.
func (s *Store) transcriptsOf(paths []string) (map[string]transcriptRow, error) {
	st, err := s.prepared()
	if err != nil {
		return nil, err
	}
	all := make(map[string]transcriptRow, len(paths))
	for _, path := range paths {
		var row transcriptRow
		err := st.transcriptRow.QueryRow(path).Scan(&row.size, &row.mtime, &row.readTo)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		all[path] = row
	}
	return all, nil
}
