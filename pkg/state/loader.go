package state

import (
	"os"
	"path/filepath"
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

const (
	DefaultParkedMaxAge = 7 * 24 * time.Hour
	parkedWindowBytes   = 64 << 10
)

type Loader struct {
	home         string
	probes       Probes
	parkedMaxAge time.Duration
	transcripts  map[string]cachedTranscript
	subagents    map[string]cachedSubagent
	parked       map[string]cachedTranscript
	mcp          claude.MCPConfig
	mcpKey       fileKey
	stats        *claude.Stats
	statsKey     fileKey
	dirs         []string
	reads        int
}

func NewLoader(home string, probes Probes) *Loader {
	return &Loader{
		home:         home,
		probes:       probes,
		parkedMaxAge: DefaultParkedMaxAge,
		transcripts:  map[string]cachedTranscript{},
		subagents:    map[string]cachedSubagent{},
		parked:       map[string]cachedTranscript{},
	}
}

func (l *Loader) WithParkedMaxAge(maxAge time.Duration) *Loader {
	l.parkedMaxAge = maxAge
	return l
}

func (l *Loader) Reads() int {
	return l.reads
}

func (l *Loader) WatchDirs() []string {
	return l.dirs
}

func (l *Loader) Load(now time.Time) (Snapshot, error) {
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

	if l.parkedMaxAge > 0 {
		live := map[string]bool{}
		for _, r := range src.Records {
			live[r.SessionID] = true
		}
		parked, skipped, err := l.catalogue(projectsDir, live, now)
		if err != nil {
			return snapshot, err
		}
		snapshot.Skipped = append(snapshot.Skipped, skipped...)
		src.Parked = parked
	}

	snapshot.Sessions = Build(src, l.probes, now)
	snapshot.Servers = Servers(snapshot.Sessions, src.MCP)
	snapshot.Skills = Skills(snapshot.Sessions)
	snapshot.Roads = Roads(snapshot.Sessions, src.Teams)
	snapshot.Teams = src.Teams
	snapshot.Power = PowerSince(append(append([]claude.Transcript{}, src.Transcripts...), src.Parked...), now.Add(-PowerWindow))
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

func (l *Loader) catalogue(projectsDir string, live map[string]bool, now time.Time) ([]claude.Transcript, []claude.SkippedFile, error) {
	paths, err := filepath.Glob(filepath.Join(projectsDir, "*", "*.jsonl"))
	if err != nil {
		return nil, nil, err
	}
	var parked []claude.Transcript
	var skipped []claude.SkippedFile
	cache := map[string]cachedTranscript{}
	cutoff := now.Add(-l.parkedMaxAge)
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
		if key.modTime.Before(cutoff) {
			continue
		}
		entry, ok := l.parked[path]
		if !ok || entry.key != key {
			l.reads++
			transcript, err := claude.ReadTranscriptWindow(path, parkedWindowBytes)
			if err != nil {
				skipped = append(skipped, claude.SkippedFile{Path: path, Err: err})
				continue
			}
			transcript.Project = filepath.Base(filepath.Dir(path))
			entry = cachedTranscript{key: key, transcript: transcript}
		}
		cache[path] = entry
		if entry.transcript.IsBridgeStub || entry.transcript.SessionID == "" || entry.transcript.LastAt.Before(cutoff) {
			continue
		}
		// The usage probe's transcripts used to be fenced out here.
		// That fence now lives in state.Build, which is the one place
		// every session passes through whichever path it came by — this
		// one only ever saw the parked half, and a probe that was still
		// running came through Records instead. Bug 54.
		parked = append(parked, entry.transcript)
	}
	l.parked = cache
	return parked, skipped, nil
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
