package state

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
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
	// reader reads a live transcript on from where it last stopped
	// (item 141).
	reader *claude.TranscriptReader
	// subagents are a stopped transcript's subagent files as last
	// looked at, and subagentsSkipped whether any could not be read,
	// so a load that trusts the watcher can reuse them (item 205).
	subagents        []string
	subagentsSkipped bool
	// gen is the last load that catalogued this stopped transcript.
	gen int
}

type cachedSubagent struct {
	key      fileKey
	subagent claude.Subagent
	skipped  []claude.SkippedFile
	// reader reads a live session's subagent on from where it last
	// stopped (item 141).
	reader *claude.SubagentReader
	gen    int
}

const stoppedWindowBytes = 64 << 10

type Loader struct {
	home        string
	probes      Probes
	transcripts map[string]cachedTranscript
	subagents   map[string]cachedSubagent
	parked      map[string]*cachedTranscript
	// parkedSubagents are stopped sessions' subagents, each read once
	// and again only when its file changes (item 90).
	parkedSubagents map[string]*cachedSubagent
	// stoppedGen counts the catalogue's loads; stoppedOut and
	// stoppedSubsOut are what it last gathered, and builtParked the
	// stopped sessions built from them, all kept while no stopped
	// transcript changes (item 205).
	stoppedGen     int
	stoppedOut     []claude.Transcript
	stoppedSubsOut map[string][]claude.Subagent
	builtParked    []Session
	mcp            claude.MCPConfig
	mcpKey         fileKey
	stats          *claude.Stats
	statsKey       fileKey
	dirs           []string
	reads          int
	groups         groupCache
	ledgerPath     string
	closeUpPath    string
	ledger         Ledger
	ledgerReads    int
	ledgerWrote    ledgerWrite
	ledgerSavedAt  time.Time
	ledgerPending  bool
	historyKey     fileKey
	historyReader  *claude.HistoryReader
	history        map[string][]claude.Prompt
	whole          *wholeRead
	stored         *storedHefts
	stoppedFiles   map[string]stoppedFile
	gate           loadGate
	// index locates transcripts without globbing every project on
	// every load (item 196).
	index *claude.Index
	// loadDone and loadTotal are the running load's sessions read and
	// found, for a city waiting on the first one (item 159).
	loadDone, loadTotal atomic.Int64
	// later reads after the first load what that load read only through
	// windows, and lane keeps the background passes to one at a time
	// (item 203).
	later *laterRead
	lane  chan struct{}
	// watch is what the daemon's watcher has said, and auditTick which
	// share of the trusted transcripts this load looks at anyway
	// (item 205).
	watch           watchState
	auditTick       int
	transcriptStats int
	loaded          bool
	firstPace       *pace
}

// firstLoadShare is the fraction of the wall clock the first load works
// for, with a background pass: it is all windows, about a second of
// parsing on a home of several hundred transcripts, and the collector's
// threads beside it took a 1 s sample to 111% of a core unpaced. Three
// quarters keeps it under one, for a quarter more of the wait before
// the map (item 203).
const firstLoadShare = 0.75

func (l *Loader) paceFirst() {
	if l.firstPace != nil {
		l.firstPace.yield(func() {})
	}
}

// LoadProgress is how many sessions a load has read, of how many it
// found: a live session counts for its record and again for its
// transcript among the stopped ones.
type LoadProgress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// LoadProgress is how far the running load, or the last, has got.
func (l *Loader) LoadProgress() LoadProgress {
	return LoadProgress{Done: int(l.loadDone.Load()), Total: int(l.loadTotal.Load())}
}

func NewLoader(home string, probes Probes) *Loader {
	return &Loader{
		home:        home,
		probes:      probes,
		transcripts: map[string]cachedTranscript{},
		subagents:   map[string]cachedSubagent{},
		parked:      map[string]*cachedTranscript{},

		parkedSubagents: map[string]*cachedSubagent{},
		index:           claude.NewIndex(),
		lane:            make(chan struct{}, 1),
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
	first := !l.loaded
	l.loaded = true
	if first && l.later != nil {
		l.firstPace = &pace{fraction: firstLoadShare, now: time.Now, sleep: time.Sleep}
		l.firstPace.start()
		defer func() { l.firstPace = nil }()
	}
	l.index.Refresh()
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
	paths := l.index.Transcripts(projectsDir)
	l.loadDone.Store(0)
	l.loadTotal.Store(int64(len(src.Records) + len(paths)))
	for _, r := range src.Records {
		l.loadDone.Add(1)
		l.paceFirst()
		path, ok := l.index.Find(projectsDir, r.SessionID)
		if !ok {
			continue
		}
		dirs = append(dirs, filepath.Dir(path), strings.TrimSuffix(path, ".jsonl"), claude.SubagentsDir(path))
		entry, err := l.transcript(path, first)
		if err != nil {
			snapshot.Skipped = append(snapshot.Skipped, claude.SkippedFile{Path: path, Err: err})
			continue
		}
		transcripts[path] = entry
		src.Transcripts = append(src.Transcripts, entry.transcript)
		if entry.transcript.IsBridgeStub {
			continue
		}
		for _, subPath := range l.index.Subagents(path) {
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
	l.transcripts, l.subagents, l.dirs = transcripts, subagents, append(dirs, projectDirs(paths)...)

	live := map[string]bool{}
	for _, r := range src.Records {
		live[r.SessionID] = true
	}
	// Every stopped session Claude Code still has a transcript for is
	// catalogued, however old: time ages a session, it never takes one
	// off the map (item 86).
	stopped, stoppedSubs, skipped, err := l.catalogue(paths, live, now.Add(-PowerWindow), first)
	if err != nil {
		return snapshot, err
	}
	snapshot.Skipped = append(snapshot.Skipped, skipped...)
	src.Stopped, src.StoppedSubagents = stopped, stoppedSubs

	if l.builtParked == nil {
		l.builtParked = buildParked(src.Stopped, src.StoppedSubagents)
	}
	snapshot.Sessions = mergeSessions(buildLive(src, l.probes, now), l.builtParked)
	if l.ledgerPath != "" {
		sessions, err := l.keepLedger(snapshot.Sessions, claudeDir, now)
		if err != nil {
			snapshot.Skipped = append(snapshot.Skipped, claude.SkippedFile{Path: l.ledgerPath, Err: err})
		}
		snapshot.Sessions = sessions
	}
	if l.later != nil {
		l.later.start()
	}
	if l.stored != nil {
		snapshot.HeftsRead, snapshot.ScaleHefts = l.stored.apply(snapshot.Sessions, projectsDir, now)
	} else {
		snapshot.HeftsRead = l.applyHefts(snapshot.Sessions, l.stoppedFiles)
	}
	snapshot.Scan = l.Progress()
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

func (l *Loader) catalogue(paths []string, live map[string]bool, powerSince time.Time, first bool) ([]claude.Transcript, map[string][]claude.Subagent, []claude.SkippedFile, error) {
	var skipped []claude.SkippedFile
	view := l.watch.take()
	tick := l.auditTick % auditEvery
	l.auditTick++
	l.stoppedGen++
	gen := l.stoppedGen
	laterBusy := l.later.busy()
	changed := l.stoppedOut == nil
	for i, path := range paths {
		l.loadDone.Add(1)
		l.paceFirst()
		id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		if live[id] {
			continue
		}
		if entry := l.parked[path]; entry != nil && i%auditEvery != tick && view.trusts(path) && !entry.subagentsSkipped && (!laterBusy || !l.later.pending(path)) {
			entry.gen = gen
			for _, subPath := range entry.subagents {
				l.parkedSubagents[subPath].gen = gen
			}
			continue
		}
		l.transcriptStats++
		key, err := stat(path)
		if err != nil {
			skipped = append(skipped, claude.SkippedFile{Path: path, Err: err})
			continue
		}
		entry := l.parked[path]
		if later, read := l.later.take(path); read && later.key == key {
			later.transcript.Project = filepath.Base(filepath.Dir(path))
			entry, changed = &cachedTranscript{key: key, transcript: later.transcript}, true
		}
		if entry == nil || entry.key != key {
			l.reads++
			transcript, err := l.stoppedTranscript(path, key, powerSince, first)
			if err != nil {
				skipped = append(skipped, claude.SkippedFile{Path: path, Err: err})
				continue
			}
			transcript.Project = filepath.Base(filepath.Dir(path))
			entry, changed = &cachedTranscript{key: key, transcript: transcript}, true
		}
		entry.gen = gen
		l.parked[path] = entry
		if entry.transcript.IsBridgeStub || entry.transcript.SessionID == "" {
			continue
		}
		subPaths, subSkipped, subChanged := l.stoppedSubagents(path, entry.subagents, gen)
		skipped = append(skipped, subSkipped...)
		entry.subagents, entry.subagentsSkipped = subPaths, len(subSkipped) > 0
		changed = changed || subChanged
	}
	for path, entry := range l.parked {
		if entry.gen != gen {
			delete(l.parked, path)
			changed = true
		}
	}
	for path, entry := range l.parkedSubagents {
		if entry.gen != gen {
			delete(l.parkedSubagents, path)
		}
	}
	if changed {
		l.stoppedOut, l.stoppedSubsOut = l.gather(paths, gen)
		l.builtParked = nil
	}
	return l.stoppedOut, l.stoppedSubsOut, skipped, nil
}

// gather is the stopped sessions catalogued in this load, in the order
// of their paths, with their subagents, and the files they came from.
func (l *Loader) gather(paths []string, gen int) ([]claude.Transcript, map[string][]claude.Subagent) {
	parked := make([]claude.Transcript, 0, len(l.parked))
	subs := map[string][]claude.Subagent{}
	files := make(map[string]stoppedFile, len(l.parked))
	for _, path := range paths {
		entry := l.parked[path]
		if entry == nil || entry.gen != gen || entry.transcript.IsBridgeStub || entry.transcript.SessionID == "" {
			continue
		}
		// The usage probe's transcripts used to be fenced out here.
		// That fence now lives in state.Build, which is the one place
		// every session passes through whichever path it came by — this
		// one only ever saw the parked half, and a probe that was still
		// running came through Records instead. Bug 54.
		parked = append(parked, entry.transcript)
		files[entry.transcript.SessionID] = stoppedFile{path: path, key: entry.key}
		for _, subPath := range entry.subagents {
			subs[entry.transcript.SessionID] = append(subs[entry.transcript.SessionID], l.parkedSubagents[subPath].subagent)
		}
	}
	l.stoppedFiles = files
	return parked, subs
}

// stoppedTranscript reads a stopped transcript through the window, and
// from the plant's window on if it was active since that opened,
// because the plant sums its hours and a window keeps only the head and
// the tail of them (item 94).
//
// With a background pass, the first load leaves that longer read to the
// later read, and every load reads the window until it has finished
// (item 203).
func (l *Loader) stoppedTranscript(path string, key fileKey, powerSince time.Time, first bool) (claude.Transcript, error) {
	transcript, err := claude.ReadTranscriptWindow(path, stoppedWindowBytes)
	if err != nil || !transcript.Partial || transcript.LastAt.Before(powerSince) {
		return transcript, err
	}
	if l.later != nil && first {
		l.later.hold(laterJob{path: path, size: key.size, since: powerSince})
	}
	if l.later.holding(path) {
		return transcript, nil
	}
	l.reads++
	return claude.ReadTranscriptSince(path, stoppedWindowBytes, powerSince)
}

// stoppedSubagents are a stopped session's subagent files, each read
// again only when it changed, and whether any is new, changed or gone
// since was the last list.
func (l *Loader) stoppedSubagents(path string, was []string, gen int) ([]string, []claude.SkippedFile, bool) {
	paths := l.index.Subagents(path)
	var kept []string
	var skipped []claude.SkippedFile
	changed := false
	for _, subPath := range paths {
		key, err := stat(subPath)
		if err != nil {
			skipped = append(skipped, claude.SkippedFile{Path: subPath, Err: err})
			continue
		}
		entry := l.parkedSubagents[subPath]
		if entry == nil || entry.key != key {
			l.reads++
			subagent, subSkipped, err := claude.ReadSubagent(subPath)
			if err != nil {
				skipped = append(skipped, claude.SkippedFile{Path: subPath, Err: err})
				continue
			}
			entry, changed = &cachedSubagent{key: key, subagent: subagent, skipped: subSkipped}, true
			l.parkedSubagents[subPath] = entry
		}
		entry.gen = gen
		kept = append(kept, subPath)
	}
	return kept, skipped, changed || !slices.Equal(kept, was)
}

// projectDirs are the directories that hold paths, each once, so the
// watcher holds a stopped session's as well as a live one's (item 205).
func projectDirs(paths []string) []string {
	var dirs []string
	for _, path := range paths {
		if dir := filepath.Dir(path); len(dirs) == 0 || dirs[len(dirs)-1] != dir {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// transcript reads a live transcript on from where its reader stopped.
// With a background pass, the first load reads one longer than a window
// through the window and leaves the whole read to the later read, whose
// reader every load after it carries on with (item 203).
func (l *Loader) transcript(path string, first bool) (cachedTranscript, error) {
	key, err := stat(path)
	if err != nil {
		return cachedTranscript{}, err
	}
	entry, ok := l.transcripts[path]
	if later, read := l.later.take(path); read && later.reader != nil {
		entry, ok = later, false
	}
	if ok && entry.key == key {
		return entry, nil
	}
	if l.later != nil && first && entry.reader == nil && key.size > 2*stoppedWindowBytes {
		l.later.hold(laterJob{path: path, size: key.size, live: true})
	}
	if entry.reader == nil && l.later.holding(path) {
		l.reads++
		transcript, err := claude.ReadTranscriptWindow(path, stoppedWindowBytes)
		if err != nil {
			return cachedTranscript{}, err
		}
		transcript.Project = filepath.Base(filepath.Dir(path))
		return cachedTranscript{key: key, transcript: transcript}, nil
	}
	reader := entry.reader
	if reader == nil {
		reader = claude.NewTranscriptReader(path)
	}
	l.reads++
	transcript, err := reader.Read()
	if err != nil {
		return cachedTranscript{}, err
	}
	transcript.Project = filepath.Base(filepath.Dir(path))
	return cachedTranscript{key: key, transcript: transcript, reader: reader}, nil
}

func (l *Loader) subagent(path string) (cachedSubagent, error) {
	key, err := stat(path)
	if err != nil {
		return cachedSubagent{}, err
	}
	entry, ok := l.subagents[path]
	if ok && entry.key == key {
		return entry, nil
	}
	reader := entry.reader
	if reader == nil {
		reader = claude.NewSubagentReader(path)
	}
	l.reads++
	subagent, skipped, err := reader.Read()
	if err != nil {
		return cachedSubagent{}, err
	}
	return cachedSubagent{key: key, subagent: subagent, skipped: skipped, reader: reader}, nil
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
// it does something after the demolish (item 145). An entry another
// harness recorded is never brought back as Claude's (item 97).
func (l *Loader) keepLedger(sessions []Session, claudeDir string, now time.Time) ([]Session, error) {
	if err := l.loadLedger(); err != nil {
		return sessions, err
	}
	shown := map[string]bool{}
	for _, s := range sessions {
		shown[s.ID] = true
	}
	sessions, changed, more := l.recordShown(sessions)
	var missing []string
	for id, e := range l.ledger {
		if !shown[id] && !e.Demolished() && e.Harness == "" {
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
	return sessions, l.settleLedger(changed, l.markPacks(sessions, now) || more, now)
}

// KeepLedger is the ledger's pass over another harness's sessions: each
// is recorded under its harness, and packed up, unpacked or kept off the
// map as a Claude session is. The Claude loader keeps the one ledger, so
// two loaders never write it (item 97).
func (l *Loader) KeepLedger(sessions []Session, now time.Time) ([]Session, []claude.SkippedFile) {
	if l.ledgerPath == "" {
		return sessions, nil
	}
	defer l.gate.hold()()
	if err := l.loadLedger(); err != nil {
		return sessions, []claude.SkippedFile{{Path: l.ledgerPath, Err: err}}
	}
	sessions, changed, more := l.recordShown(sessions)
	if err := l.settleLedger(changed, l.markPacks(sessions, now) || more, now); err != nil {
		return sessions, []claude.SkippedFile{{Path: l.ledgerPath, Err: err}}
	}
	return sessions, nil
}

// recordShown records each session shown, and drops each you demolished
// that has done nothing since.
func (l *Loader) recordShown(sessions []Session) ([]Session, bool, bool) {
	changed, more := false, false
	kept := sessions[:0]
	for _, s := range sessions {
		if e := l.ledger[s.ID]; e.Demolished() {
			if !s.LastActivity.After(e.DemolishedAt) {
				continue
			}
			e.DemolishedAt = time.Time{}
			l.ledger[s.ID] = e
			changed, more = true, true
		}
		c, m := l.ledger.record(s)
		changed, more = changed || c, more || m
		kept = append(kept, s)
	}
	return kept, changed, more
}

// markPacks packs up and unpacks what has merged or moved on, and marks
// each session packed as the ledger has it.
func (l *Loader) markPacks(sessions []Session, now time.Time) bool {
	changed := false
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
	return changed
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
// file has changed, and then only the lines appended since (item 158).
func (l *Loader) promptHistory(path string) (map[string][]claude.Prompt, error) {
	key, err := stat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if l.history != nil && key == l.historyKey {
		return l.history, nil
	}
	if l.historyReader == nil {
		l.historyReader = claude.NewHistoryReader()
	}
	history, err := l.historyReader.Read(path)
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
	return l.saveLedger()
}
