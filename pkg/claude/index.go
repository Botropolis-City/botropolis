package claude

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// tick is the coarsest modification-time granularity a directory's
// listing allows for: a listing taken within it of the directory's last
// change could have missed an entry added in the same tick, so it is
// taken again until the directory has been quiet that long.
const tick = 2 * time.Second

// Index locates transcripts and their subagents without globbing: each
// directory is listed once and again only when its modification time
// moves, so a settled rescan stats the directories instead of reading
// them. Item 196.
type Index struct {
	mu       sync.Mutex
	dirs     map[string]listing
	listings int
	stats    int
	// gen is the current load's; a directory looked at in it is not
	// looked at again until Refresh.
	gen int
	// versions counts listings read, and found is the last answer of
	// Transcripts with the version of each listing it came from, kept
	// while none of them is read again (item 205).
	versions int
	found    map[string]foundTranscripts
}

type foundTranscripts struct {
	versions []int
	paths    []string
}

type listing struct {
	mtime   time.Time
	trusted bool
	seen    int
	version int
	files   []string
	subdirs map[string]bool
}

func NewIndex() *Index {
	return &Index{dirs: map[string]listing{}, found: map[string]foundTranscripts{}}
}

// Refresh starts a new load: from now on each directory is looked at
// once more, and then not again until the next Refresh. Without it,
// every lookup looks at the directory.
func (ix *Index) Refresh() {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.gen++
}

// Listings is how many times a directory has been read.
func (ix *Index) Listings() int {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	return ix.listings
}

// Stats is how many times a directory's modification time has been
// looked at.
func (ix *Index) Stats() int {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	return ix.stats
}

// Transcripts is every main transcript under projectsDir, sorted, as
// filepath.Glob(projectsDir/*/*.jsonl) gives them.
// The answer is shared from one call to the next while no directory
// it came from has changed, so a caller must not change it.
func (ix *Index) Transcripts(projectsDir string) []string {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	projects := ix.subdirs(projectsDir)
	versions := make([]int, 0, len(projects)+1)
	versions = append(versions, ix.dirs[projectsDir].version)
	for _, project := range projects {
		versions = append(versions, ix.list(filepath.Join(projectsDir, project)).version)
	}
	if found, ok := ix.found[projectsDir]; ok && slices.Equal(found.versions, versions) {
		return found.paths
	}
	var paths []string
	for _, project := range projects {
		dir := filepath.Join(projectsDir, project)
		for _, name := range ix.list(dir).files {
			if strings.HasSuffix(name, ".jsonl") {
				paths = append(paths, filepath.Join(dir, name))
			}
		}
	}
	sort.Strings(paths)
	ix.found[projectsDir] = foundTranscripts{versions: versions, paths: paths}
	return paths
}

// Find is the transcript of sessionID, from the first project in order
// that holds one, as FindTranscript finds it.
func (ix *Index) Find(projectsDir, sessionID string) (string, bool) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	name := sessionID + ".jsonl"
	for _, project := range ix.subdirs(projectsDir) {
		dir := filepath.Join(projectsDir, project)
		if _, ok := slicesIndex(ix.list(dir).files, name); ok {
			return filepath.Join(dir, name), true
		}
	}
	return "", false
}

// Subagents is a session's subagent transcripts, sorted, as
// SubagentPaths gives them.
func (ix *Index) Subagents(transcriptPath string) []string {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	project, session := filepath.Dir(transcriptPath), strings.TrimSuffix(filepath.Base(transcriptPath), ".jsonl")
	if !ix.list(project).subdirs[session] {
		return nil
	}
	return ix.agents(filepath.Join(project, session))
}

func (ix *Index) subdirs(dir string) []string {
	l := ix.list(dir)
	names := make([]string, 0, len(l.subdirs))
	for name := range l.subdirs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// list is dir's entries, from the last listing while the directory's
// modification time has not moved since and it was taken a tick after
// the directory last changed; a directory that is not there has none.
func (ix *Index) list(dir string) listing {
	l, ok := ix.dirs[dir]
	if ok && ix.gen > 0 && l.seen == ix.gen {
		return l
	}
	ix.stats++
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		delete(ix.dirs, dir)
		return listing{}
	}
	if ok && l.trusted && l.mtime.Equal(info.ModTime()) {
		l.seen = ix.gen
		ix.dirs[dir] = l
		return l
	}
	listedAt := time.Now()
	entries, err := os.ReadDir(dir)
	ix.listings++
	if err != nil {
		delete(ix.dirs, dir)
		return listing{}
	}
	ix.versions++
	l = listing{mtime: info.ModTime(), trusted: listedAt.Sub(info.ModTime()) > tick, seen: ix.gen, version: ix.versions, subdirs: map[string]bool{}}
	for _, e := range entries {
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			target, err := os.Stat(filepath.Join(dir, e.Name()))
			isDir = err == nil && target.IsDir()
		}
		if isDir {
			l.subdirs[e.Name()] = true
		} else {
			l.files = append(l.files, e.Name())
		}
	}
	sort.Strings(l.files)
	ix.dirs[dir] = l
	return l
}

func slicesIndex(sorted []string, name string) (int, bool) {
	i := sort.SearchStrings(sorted, name)
	return i, i < len(sorted) && sorted[i] == name
}

// SubagentTranscripts is every subagent transcript under projectsDir,
// sorted, as filepath.Glob(projectsDir/*/*/subagents/agent-*.jsonl)
// gives them: a session's whose main transcript is gone included.
func (ix *Index) SubagentTranscripts(projectsDir string) []string {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	var paths []string
	for _, project := range ix.subdirs(projectsDir) {
		projectDir := filepath.Join(projectsDir, project)
		for _, session := range ix.subdirs(projectDir) {
			paths = append(paths, ix.agents(filepath.Join(projectDir, session))...)
		}
	}
	sort.Strings(paths)
	return paths
}

// agents is the subagent transcripts in a session's directory.
func (ix *Index) agents(sessionDir string) []string {
	if !ix.list(sessionDir).subdirs["subagents"] {
		return nil
	}
	dir := filepath.Join(sessionDir, "subagents")
	var paths []string
	for _, name := range ix.list(dir).files {
		if strings.HasPrefix(name, agentFilePrefix) && strings.HasSuffix(name, ".jsonl") {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	return paths
}
