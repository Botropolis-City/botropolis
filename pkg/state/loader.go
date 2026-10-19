package state

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

type fileKey struct {
	size    int64
	modTime time.Time
}

type cachedTranscript struct {
	key        fileKey
	transcript claude.Transcript
}

type cachedSubagent struct {
	key      fileKey
	subagent claude.Subagent
	skipped  []claude.SkippedFile
}

const stoppedWindowBytes = 64 << 10

type Loader struct {
	home        string
	probes      Probes
	transcripts map[string]cachedTranscript
	subagents   map[string]cachedSubagent
	parked      map[string]cachedTranscript
	// parkedSubagents are stopped sessions' subagents, each read once
	// and again only when its file changes (item 90).
	parkedSubagents map[string]cachedSubagent
	mcp             claude.MCPConfig
	mcpKey          fileKey
	stats           *claude.Stats
	statsKey        fileKey
	dirs            []string
	reads           int
	groups          groupCache
	ledgerPath      string
	closeUpPath     string
	ledger          Ledger
	historyKey      fileKey
	history         map[string][]claude.Prompt
	whole           *wholeRead
	stored          *storedHefts
	stoppedFiles    map[string]stoppedFile
	gate            loadGate
}

func NewLoader(home string, probes Probes) *Loader {
	return &Loader{
		home:        home,
		probes:      probes,
		transcripts: map[string]cachedTranscript{},
		subagents:   map[string]cachedSubagent{},
		parked:      map[string]cachedTranscript{},

		parkedSubagents: map[string]cachedSubagent{},
	}
}

// WithLedger keeps a ledger of every session shown at path, and brings
// back from it any session whose transcript Claude Code has deleted.
// Item 86.
func (l *Loader) WithLedger(path string) *Loader {
	l.ledgerPath = path
	return l
}

func (l *Loader) Reads() int {
	return l.reads
}

func (l *Loader) WatchDirs() []string {
	return l.dirs
}

func (l *Loader) Load(now time.Time) (Snapshot, error) {
	defer l.gate.hold()()
	claudeDir := filepath.Join(l.home, ".claude")
	projectsDir := filepath.Join(claudeDir, "projects")
	snapshot := Snapshot{At: now}
	var src Sources
	var err error

	var skipped []claude.SkippedFile
	if src.Records, skipped, err = claude.ReadSessionRecords(filepath.Join(claudeDir, "sessions")); err != nil {
		return snapshot, err
	}
	snapshot.Skipped = append(snapshot.Skipped, skipped...)

	if src.Roster, err = claude.ReadRoster(filepath.Join(claudeDir, "daemon", "roster.json")); err != nil {
		return snapshot, err
	}
	if src.MCP, err = l.mcpConfig(filepath.Join(l.home, ".claude.json")); err != nil {
		snapshot.Skipped = append(snapshot.Skipped, claude.SkippedFile{Path: filepath.Join(l.home, ".claude.json"), Err: err})
	}
	if snapshot.Stats, err = l.statsCache(filepath.Join(claudeDir, "stats-cache.json")); err != nil {
		snapshot.Skipped = append(snapshot.Skipped, claude.SkippedFile{Path: filepath.Join(claudeDir, "stats-cache.json"), Err: err})
	}
	if src.Teams, skipped, err = claude.ReadTeams(filepath.Join(claudeDir, "teams")); err != nil {
		return snapshot, err
	}
	snapshot.Skipped = append(snapshot.Skipped, skipped...)

	transcripts := map[string]cachedTranscript{}
	subagents := map[string]cachedSubagent{}
	src.Subagents = map[string][]claude.Subagent{}
	dirs := []string{filepath.Join(claudeDir, "sessions"), filepath.Join(claudeDir, "daemon"), projectsDir}
	for _, r := range src.Records {
		path, ok := claude.FindTranscript(projectsDir, r.SessionID)
		if !ok {
			continue
		}
		dirs = append(dirs, filepath.Dir(path), strings.TrimSuffix(path, ".jsonl"), claude.SubagentsDir(path))
		entry, err := l.transcript(path)
		if err != nil {
			snapshot.Skipped = append(snapshot.Skipped, claude.SkippedFile{Path: path, Err: err})
			continue
		}
		transcripts[path] = entry
		src.Transcripts = append(src.Transcripts, entry.transcript)
		if entry.transcript.IsBridgeStub {
			continue
		}
		paths, err := claude.SubagentPaths(path)
		if err != nil {
			return snapshot, err
		}
		for _, subPath := range paths {
			sub, err := l.subagent(subPath)
			if err != nil {
				snapshot.Skipped = append(snapshot.Skipped, claude.SkippedFile{Path: subPath, Err: err})
				continue
			}
			subagents[subPath] = sub
			snapshot.Skipped = append(snapshot.Skipped, sub.skipped...)
			src.Subagents[r.SessionID] = append(src.Subagents[r.SessionID], sub.subagent)
		}
	}
	l.transcripts, l.subagents, l.dirs = transcripts, subagents, dirs

	live := map[string]bool{}
	for _, r := range src.Records {
		live[r.SessionID] = true
	}
	// Every stopped session Claude Code still has a transcript for is
	// catalogued, however old: time ages a session, it never takes one
	// off the map (item 86).
	stopped, stoppedSubs, skipped, err := l.catalogue(projectsDir, live, now.Add(-PowerWindow))
	if err != nil {
		return snapshot, err
	}
	snapshot.Skipped = append(snapshot.Skipped, skipped...)
	src.Stopped, src.StoppedSubagents = stopped, stoppedSubs

	snapshot.Sessions = Build(src, l.probes, now)
	if l.ledgerPath != "" {
		sessions, err := l.keepLedger(snapshot.Sessions, claudeDir, now)
		if err != nil {
			snapshot.Skipped = append(snapshot.Skipped, claude.SkippedFile{Path: l.ledgerPath, Err: err})
		}
		snapshot.Sessions = sessions
	}
	if l.stored != nil {
		snapshot.HeftsRead, snapshot.ScaleHefts = l.stored.apply(snapshot.Sessions, projectsDir, now)
	} else {
		snapshot.HeftsRead = l.applyHefts(snapshot.Sessions, l.stoppedFiles)
	}
	snapshot.Servers = Servers(snapshot.Sessions, src.MCP)
	snapshot.Skills = Skills(snapshot.Sessions)
	snapshot.Roads = Roads(snapshot.Sessions, src.Teams)
	snapshot.Teams = src.Teams
	if err := l.applyGroups(&snapshot, claudeDir, now); err != nil {
		snapshot.Skipped = append(snapshot.Skipped, claude.SkippedFile{Path: filepath.Join(claudeDir, "history.jsonl"), Err: err})
	}
	snapshot.Power = PowerSince(append(append([]claude.Transcript{}, src.Transcripts...), src.Stopped...), now.Add(-PowerWindow))
	return snapshot, nil
}

func (l *Loader) mcpConfig(path string) (claude.MCPConfig, error) {
	key, err := stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return claude.MCPConfig{}, nil
		}
		return claude.MCPConfig{}, err
	}
	if l.mcpKey == key {
		return l.mcp, nil
	}
	mcp, err := claude.ReadMCPConfig(path)
	if err != nil {
		return claude.MCPConfig{}, err
	}
	l.mcp, l.mcpKey = mcp, key
	return mcp, nil
}

func (l *Loader) statsCache(path string) (*claude.Stats, error) {
	key, err := stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if l.statsKey == key {
		return l.stats, nil
	}
	stats, err := claude.ReadStats(path)
	if err != nil {
		return nil, err
	}
	l.stats, l.statsKey = &stats, key
	return l.stats, nil
}

func (l *Loader) catalogue(projectsDir string, live map[string]bool, powerSince time.Time) ([]claude.Transcript, map[string][]claude.Subagent, []claude.SkippedFile, error) {
	paths, err := filepath.Glob(filepath.Join(projectsDir, "*", "*.jsonl"))
	if err != nil {
		return nil, nil, nil, err
	}
	var parked []claude.Transcript
	var skipped []claude.SkippedFile
	cache := map[string]cachedTranscript{}
	subCache := map[string]cachedSubagent{}
	stoppedSubs := map[string][]claude.Subagent{}
	files := map[string]stoppedFile{}
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		if live[id] {
			continue
		}
		key, err := stat(path)
		if err != nil {
			skipped = append(skipped, claude.SkippedFile{Path: path, Err: err})
			continue
		}
		entry, ok := l.parked[path]
		if !ok || entry.key != key {
			l.reads++
			transcript, err := l.stoppedTranscript(path, powerSince)
			if err != nil {
				skipped = append(skipped, claude.SkippedFile{Path: path, Err: err})
				continue
			}
			transcript.Project = filepath.Base(filepath.Dir(path))
			entry = cachedTranscript{key: key, transcript: transcript}
		}
		cache[path] = entry
		if entry.transcript.IsBridgeStub || entry.transcript.SessionID == "" {
			continue
		}
		// The usage probe's transcripts used to be fenced out here.
		// That fence now lives in state.Build, which is the one place
		// every session passes through whichever path it came by — this
		// one only ever saw the parked half, and a probe that was still
		// running came through Records instead. Bug 54.
		parked = append(parked, entry.transcript)
		files[entry.transcript.SessionID] = stoppedFile{path: path, key: key}
		subs, subSkipped := l.stoppedSubagents(path, subCache)
		skipped = append(skipped, subSkipped...)
		if len(subs) > 0 {
			stoppedSubs[entry.transcript.SessionID] = subs
		}
	}
	l.parked, l.parkedSubagents, l.stoppedFiles = cache, subCache, files
	return parked, stoppedSubs, skipped, nil
}

// stoppedTranscript reads a stopped transcript through the window, and
// from the plant's window on if it was active since that opened,
// because the plant sums its hours and a window keeps only the head and
// the tail of them (item 94).
func (l *Loader) stoppedTranscript(path string, powerSince time.Time) (claude.Transcript, error) {
	transcript, err := claude.ReadTranscriptWindow(path, stoppedWindowBytes)
	if err != nil || !transcript.Partial || transcript.LastAt.Before(powerSince) {
		return transcript, err
	}
	l.reads++
	return claude.ReadTranscriptSince(path, stoppedWindowBytes, powerSince)
}

// stoppedSubagents are a stopped session's subagents, from the cache
// unless a file changed, kept into next for the following load.
func (l *Loader) stoppedSubagents(path string, next map[string]cachedSubagent) ([]claude.Subagent, []claude.SkippedFile) {
	paths, err := claude.SubagentPaths(path)
	if err != nil {
		return nil, []claude.SkippedFile{{Path: path, Err: err}}
	}
	var subs []claude.Subagent
	var skipped []claude.SkippedFile
	for _, subPath := range paths {
		key, err := stat(subPath)
		if err != nil {
			skipped = append(skipped, claude.SkippedFile{Path: subPath, Err: err})
			continue
		}
		entry, ok := l.parkedSubagents[subPath]
		if !ok || entry.key != key {
			l.reads++
			subagent, subSkipped, err := claude.ReadSubagent(subPath)
			if err != nil {
				skipped = append(skipped, claude.SkippedFile{Path: subPath, Err: err})
				continue
			}
			entry = cachedSubagent{key: key, subagent: subagent, skipped: subSkipped}
		}
		next[subPath] = entry
		subs = append(subs, entry.subagent)
	}
	return subs, skipped
}

func (l *Loader) transcript(path string) (cachedTranscript, error) {
	key, err := stat(path)
	if err != nil {
		return cachedTranscript{}, err
	}
	if entry, ok := l.transcripts[path]; ok && entry.key == key {
		return entry, nil
	}
	l.reads++
	transcript, err := claude.ReadTranscript(path)
	if err != nil {
		return cachedTranscript{}, err
	}
	transcript.Project = filepath.Base(filepath.Dir(path))
	return cachedTranscript{key: key, transcript: transcript}, nil
}

func (l *Loader) subagent(path string) (cachedSubagent, error) {
	key, err := stat(path)
	if err != nil {
		return cachedSubagent{}, err
	}
	if entry, ok := l.subagents[path]; ok && entry.key == key {
		return entry, nil
	}
	l.reads++
	subagent, skipped, err := claude.ReadSubagent(path)
	if err != nil {
		return cachedSubagent{}, err
	}
	return cachedSubagent{key: key, subagent: subagent, skipped: skipped}, nil
}

func stat(path string) (fileKey, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileKey{}, err
	}
	return fileKey{size: info.Size(), modTime: info.ModTime()}, nil
}

// keepLedger records every session shown and adds back each one the
// ledger knows that nothing else brought: its transcript is gone. A
// session you demolished stays off, whatever transcript remains, until
// it does something after the demolish (item 145).
func (l *Loader) keepLedger(sessions []Session, claudeDir string, now time.Time) ([]Session, error) {
	if err := l.loadLedger(); err != nil {
		return sessions, err
	}
	changed := false
	shown := map[string]bool{}
	kept := sessions[:0]
	for _, s := range sessions {
		shown[s.ID] = true
		if e := l.ledger[s.ID]; e.Demolished() {
			if !s.LastActivity.After(e.DemolishedAt) {
				continue
			}
			e.DemolishedAt = time.Time{}
			l.ledger[s.ID] = e
			changed = true
		}
		if l.ledger.Record(s) {
			changed = true
		}
		kept = append(kept, s)
	}
	sessions = kept
	var missing []string
	for id, e := range l.ledger {
		if !shown[id] && !e.Demolished() {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		history, err := l.promptHistory(filepath.Join(claudeDir, "history.jsonl"))
		if err != nil {
			return sessions, err
		}
		sort.Strings(missing)
		for _, id := range missing {
			sessions = append(sessions, goneSession(id, l.ledger[id], history[id]))
		}
	}
	for i := range sessions {
		s := &sessions[i]
		if l.packOnMerge(*s, now) {
			changed = true
		}
		if l.unpackOnWork(*s, now) {
			changed = true
		}
		e := l.ledger[s.ID]
		s.Packed = e.Packed()
		if s.Packed {
			s.PackedAt = e.PackedAt
		}
	}
	if changed {
		return sessions, l.ledger.Save(l.ledgerPath)
	}
	return sessions, nil
}

// packOnMerge packs up a stopped session whose pull request has merged,
// unless you unpacked it since its last activity: a merge ends the work,
// and an unpack says it has not ended.
func (l *Loader) packOnMerge(s Session, now time.Time) bool {
	e := l.ledger[s.ID]
	if s.State != Resting || e.Packed() || !merged(s.PRs) {
		return false
	}
	if !e.UnpackedAt.IsZero() && !s.LastActivity.After(e.UnpackedAt) {
		return false
	}
	l.ledger.Pack(s.ID, now)
	return true
}

// unpackOnWork brings back a packed-up session that has done something
// since it was packed: work that resumes is never kept in the depot.
func (l *Loader) unpackOnWork(s Session, now time.Time) bool {
	e := l.ledger[s.ID]
	if !e.Packed() || !s.LastActivity.After(e.PackedAt) {
		return false
	}
	l.ledger.Unpack(s.ID, now)
	return true
}

func merged(prs []claude.PR) bool {
	for _, pr := range prs {
		if pr.Merged() {
			return true
		}
	}
	return false
}

// promptHistory reads Claude Code's prompt history again only when the
// file has changed.
func (l *Loader) promptHistory(path string) (map[string][]claude.Prompt, error) {
	key, err := stat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if l.history != nil && key == l.historyKey {
		return l.history, nil
	}
	history, err := claude.ReadPromptHistory(path)
	if err != nil {
		return nil, err
	}
	l.history, l.historyKey = history, key
	return history, nil
}

// ErrNoLedger is a pack or unpack asked of a loader that keeps no
// ledger, so has nowhere to write it.
var ErrNoLedger = errors.New("this daemon keeps no ledger, so it cannot pack sessions up")

// Pack packs a session up, or unpacks it, in the ledger (item 86).
func (l *Loader) Pack(id string, packed bool, at time.Time) error {
	if l.ledgerPath == "" {
		return ErrNoLedger
	}
	if err := l.loadLedger(); err != nil {
		return err
	}
	if packed {
		l.ledger.Pack(id, at)
	} else {
		l.ledger.Unpack(id, at)
	}
	return l.ledger.Save(l.ledgerPath)
}
