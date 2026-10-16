package claude

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/measure"
)

const (
	maxTranscriptLine = 64 << 20
	sendMessageTool   = "SendMessage"
)

type Usage struct {
	Input       int64 `json:"input"`
	Output      int64 `json:"output"`
	CacheRead   int64 `json:"cacheRead"`
	CacheCreate int64 `json:"cacheCreate"`
	Thinking    int64 `json:"thinking"`
	Messages    int   `json:"messages"`
	// Partial is usage read through a head and a tail window rather than
	// the whole session, so it is not the session's total (item 86), the
	// way costKnown marks a cost (item 6). A Codex rollout's token counts
	// are cumulative and stay exact; its message count does not.
	Partial bool `json:"partial,omitempty"`
}

func (u Usage) Add(other Usage) Usage {
	return Usage{
		Input:       u.Input + other.Input,
		Output:      u.Output + other.Output,
		CacheRead:   u.CacheRead + other.CacheRead,
		CacheCreate: u.CacheCreate + other.CacheCreate,
		Thinking:    u.Thinking + other.Thinking,
		Messages:    u.Messages + other.Messages,

		Partial: u.Partial || other.Partial,
	}
}

func (u Usage) Context() int64 {
	return u.Input + u.CacheRead + u.CacheCreate
}

type ModelCost struct {
	Usage Usage
	USD   float64
}

type Cost struct {
	TotalUSD float64
	Models   map[string]ModelCost
	// Known is whether a cost-state record was seen at all; without one
	// the cost is unknown, not zero.
	Known bool
}

type PR struct {
	Number     int    `json:"number"`
	URL        string `json:"url"`
	Repository string `json:"repository"`
	// State is open, merged or closed, from the last pr action seen.
	State string `json:"state,omitempty"`
}

const (
	PROpen   = "open"
	PRMerged = "merged"
	PRClosed = "closed"
)

// Merged reports whether the PR was merged.
func (p PR) Merged() bool {
	return p.State == PRMerged
}

type Transcript struct {
	Path       string
	Project    string
	SessionID  string
	CWD        string
	Branch     string
	Version    string
	Entrypoint string
	Title      string
	Model      string
	ModelID    string
	Effort     string
	Usage      Usage
	// Hourly is Usage bucketed by unix hour, so a window over recent
	// activity can be summed without rescanning.
	Hourly           map[int64]Usage
	ContextTokens    int64
	MaxContext       int64
	Cost             Cost
	Tail             Tail
	TeamName         string
	AgentName        string
	Messages         map[string]int
	Touches          map[string]int
	MCPCalls         map[string]int
	SkillCalls       map[string]int
	PRs              []PR
	APIErrors        int
	LastErrorAt      time.Time
	Compactions      int
	LastCompactionAt time.Time
	FirstAt          time.Time
	LastAt           time.Time
	// Recap is the newest away summary Claude Code wrote into the
	// transcript, and LastPrompt the newest last-prompt record.
	Recap        Recap
	LastPrompt   string
	IsBridgeStub bool
	Partial      bool
	Malformed    int
}

// Recap is the summary Claude Code shows at the foot of a session left
// waiting: when it was written, and how many prompts the session has
// taken since, so its age can be stated rather than implied.
type Recap struct {
	Text         string    `json:"text"`
	At           time.Time `json:"at"`
	PromptsSince int       `json:"promptsSince"`
}

type usageJSON struct {
	Input       int64 `json:"input_tokens"`
	Output      int64 `json:"output_tokens"`
	CacheRead   int64 `json:"cache_read_input_tokens"`
	CacheCreate int64 `json:"cache_creation_input_tokens"`
	Details     struct {
		Thinking int64 `json:"thinking_tokens"`
	} `json:"output_tokens_details"`
}

func (u usageJSON) usage() Usage {
	return Usage{
		Input:       u.Input,
		Output:      u.Output,
		CacheRead:   u.CacheRead,
		CacheCreate: u.CacheCreate,
		Thinking:    u.Details.Thinking,
		Messages:    1,
	}
}

type modelUsageJSON struct {
	Input       int64   `json:"inputTokens"`
	Output      int64   `json:"outputTokens"`
	Thinking    int64   `json:"thinkingTokens"`
	CacheRead   int64   `json:"cacheReadInputTokens"`
	CacheCreate int64   `json:"cacheCreationInputTokens"`
	CostUSD     float64 `json:"costUSD"`
}

func (mu modelUsageJSON) modelCost() ModelCost {
	return ModelCost{
		USD: mu.CostUSD,
		Usage: Usage{
			Input:       mu.Input,
			Output:      mu.Output,
			CacheRead:   mu.CacheRead,
			CacheCreate: mu.CacheCreate,
			Thinking:    mu.Thinking,
		},
	}
}

// prActionJSON is the pr object the CLI writes on a record when a pull
// request is created, commented on, edited, closed or merged.
type prActionJSON struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Action string `json:"action"`
}

type transcriptLineJSON struct {
	Type         string                    `json:"type"`
	SessionID    string                    `json:"sessionId"`
	CWD          string                    `json:"cwd"`
	GitBranch    string                    `json:"gitBranch"`
	Version      string                    `json:"version"`
	Entrypoint   string                    `json:"entrypoint"`
	Effort       string                    `json:"effort"`
	AITitle      string                    `json:"aiTitle"`
	CustomTitle  string                    `json:"customTitle"`
	Summary      string                    `json:"summary"`
	LastPrompt   string                    `json:"lastPrompt"`
	Content      json.RawMessage           `json:"content"`
	Timestamp    string                    `json:"timestamp"`
	Subtype      string                    `json:"subtype"`
	IsSidechain  bool                      `json:"isSidechain"`
	IsMeta       bool                      `json:"isMeta"`
	IsAPIError   bool                      `json:"isApiErrorMessage"`
	MCPServer    string                    `json:"attributionMcpServer"`
	Skill        string                    `json:"attributionSkill"`
	PRNumber     int                       `json:"prNumber"`
	PRURL        string                    `json:"prUrl"`
	PRRepository string                    `json:"prRepository"`
	PRAction     *prActionJSON             `json:"pr"`
	TeamName     string                    `json:"teamName"`
	AgentName    string                    `json:"agentName"`
	Compact      json.RawMessage           `json:"compactMetadata"`
	TotalCostUSD float64                   `json:"totalCostUSD"`
	ModelUsage   map[string]modelUsageJSON `json:"modelUsage"`
	Attachment   struct {
		Type     string `json:"type"`
		Identity struct {
			ModelID string `json:"modelId"`
		} `json:"identity"`
	} `json:"attachment"`
	Message struct {
		ID         string      `json:"id"`
		Model      string      `json:"model"`
		StopReason string      `json:"stop_reason"`
		Usage      *usageJSON  `json:"usage"`
		Content    contentJSON `json:"content"`
	} `json:"message"`
}

type titleRank int

const (
	titleNone titleRank = iota
	titleSummary
	titleAI
	titleCustom
)

type transcriptScan struct {
	transcript    Transcript
	rank          titleRank
	records       int
	bridgeRecords int
	seenMessages  map[string]bool
	tail          tailScan
	sidechainMain bool
	// recapPrompts is the prompt count when the newest recap was written.
	recapPrompts int
}

func ReadTranscript(path string) (Transcript, error) {
	return readTranscriptFile(path, false)
}

func readTranscriptFile(path string, sidechainIsMain bool) (Transcript, error) {
	f, err := openTranscript(path)
	if err != nil {
		return Transcript{}, err
	}
	defer func() { _ = f.Close() }()
	scan := newTranscriptScan(path, sidechainIsMain)
	scanLines(scan, f)
	return scan.finish(), nil
}

func (s *transcriptScan) apply(rec transcriptLineJSON) {
	t := &s.transcript
	s.records++
	if rec.Type == "bridge-session" {
		s.bridgeRecords++
	}
	if t.SessionID == "" {
		t.SessionID = rec.SessionID
	}
	if t.CWD == "" {
		t.CWD = rec.CWD
	}
	if t.Entrypoint == "" {
		t.Entrypoint = rec.Entrypoint
	}
	if rec.GitBranch != "" && rec.GitBranch != "HEAD" {
		t.Branch = rec.GitBranch
	}
	if rec.Version != "" {
		t.Version = rec.Version
	}
	if rec.TeamName != "" {
		t.TeamName = rec.TeamName
	}
	if rec.AgentName != "" {
		t.AgentName = rec.AgentName
	}
	ts, hasTime := parseTimestamp(rec.Timestamp)
	if hasTime {
		if t.FirstAt.IsZero() {
			t.FirstAt = ts
		}
		t.LastAt = ts
	}
	mainLine := !rec.IsSidechain || s.sidechainMain
	switch rec.Type {
	case "user":
		if mainLine {
			s.tail.user(rec, ts)
		}
	case "assistant":
		s.applyAssistant(rec, mainLine)
		if mainLine {
			s.tail.assistant(rec, ts)
		}
		if rec.IsAPIError {
			t.APIErrors++
			t.LastErrorAt = ts
		}
	case "system":
		if rec.Subtype == "api_error" {
			t.APIErrors++
			t.LastErrorAt = ts
		}
		if len(rec.Compact) > 0 {
			t.Compactions++
			t.LastCompactionAt = ts
		}
		if rec.Subtype == "away_summary" {
			s.applyRecap(rec, ts)
		}
	case "last-prompt":
		if rec.LastPrompt != "" {
			t.LastPrompt = rec.LastPrompt
		}
	case "pr-link":
		s.applyPR(rec)
	}
	if rec.PRAction != nil {
		s.applyPRAction(rec)
	}
	switch rec.Type {
	case "cost-state":
		s.applyCost(rec)
	case "attachment":
		if rec.Attachment.Type == "model" && rec.Attachment.Identity.ModelID != "" {
			t.ModelID = rec.Attachment.Identity.ModelID
		}
	default:
		s.applyTitle(rec)
	}
}

func (s *transcriptScan) applyRecap(rec transcriptLineJSON, ts time.Time) {
	var text string
	if err := json.Unmarshal(rec.Content, &text); err != nil || text == "" {
		return
	}
	s.transcript.Recap = Recap{Text: text, At: ts}
	s.recapPrompts = s.tail.tail.Prompts
}

func parseTimestamp(raw string) (time.Time, bool) {
	ts, err := time.Parse(time.RFC3339Nano, raw)
	return ts, err == nil
}

func (s *transcriptScan) applyAssistant(rec transcriptLineJSON, mainLine bool) {
	t := &s.transcript
	if rec.Message.Model != "" {
		t.Model = rec.Message.Model
	}
	if rec.Effort != "" {
		t.Effort = rec.Effort
	}
	if rec.Message.Usage != nil && mainLine {
		t.ContextTokens = rec.Message.Usage.usage().Context()
		if t.ContextTokens > t.MaxContext {
			t.MaxContext = t.ContextTokens
		}
	}
	if id := rec.Message.ID; id != "" {
		if s.seenMessages[id] {
			return
		}
		s.seenMessages[id] = true
	}
	if rec.Message.Usage != nil {
		u := rec.Message.Usage.usage()
		t.Usage = t.Usage.Add(u)
		if at, ok := parseTimestamp(rec.Timestamp); ok {
			if t.Hourly == nil {
				t.Hourly = map[int64]Usage{}
			}
			hour := at.Unix() / 3600
			t.Hourly[hour] = t.Hourly[hour].Add(u)
		}
	}
	for _, block := range rec.Message.Content {
		if block.Type != "tool_use" {
			continue
		}
		if block.Name == sendMessageTool && block.Input.To != "" {
			if t.Messages == nil {
				t.Messages = map[string]int{}
			}
			t.Messages[block.Input.To]++
		}
		if path := foreignFile(t.CWD, block); path != "" {
			if t.Touches == nil {
				t.Touches = map[string]int{}
			}
			t.Touches[path]++
		}
	}
	if rec.MCPServer != "" {
		if t.MCPCalls == nil {
			t.MCPCalls = map[string]int{}
		}
		t.MCPCalls[rec.MCPServer]++
	}
	if rec.Skill != "" {
		if t.SkillCalls == nil {
			t.SkillCalls = map[string]int{}
		}
		t.SkillCalls[rec.Skill]++
	}
}

var fileTools = map[string]bool{"Read": true, "Edit": true, "MultiEdit": true, "Write": true, "NotebookEdit": true}

// foreignFile is the file a tool_use block touches when it lies outside
// the session's own project, or "" when it is local or not a file tool.
func foreignFile(cwd string, block contentBlockJSON) string {
	if cwd == "" || !fileTools[block.Name] {
		return ""
	}
	path := block.Input.FilePath
	if path == "" {
		path = block.Input.NotebookPath
	}
	if path == "" || path == cwd || strings.HasPrefix(path, cwd+"/") {
		return ""
	}
	return path
}

func (s *transcriptScan) applyPR(rec transcriptLineJSON) {
	if rec.PRURL == "" {
		return
	}
	for i, pr := range s.transcript.PRs {
		if pr.URL == rec.PRURL {
			s.transcript.PRs[i] = PR{Number: rec.PRNumber, URL: rec.PRURL, Repository: rec.PRRepository, State: pr.State}
			return
		}
	}
	s.transcript.PRs = append(s.transcript.PRs, PR{Number: rec.PRNumber, URL: rec.PRURL, Repository: rec.PRRepository, State: PROpen})
}

// applyPRAction follows a PR through its life: created opens it, merged
// and closed end it; anything else leaves the state alone.
func (s *transcriptScan) applyPRAction(rec transcriptLineJSON) {
	a := rec.PRAction
	state := ""
	switch a.Action {
	case "created", "ready":
		state = PROpen
	case "merged":
		state = PRMerged
	case "closed":
		state = PRClosed
	}
	for i, pr := range s.transcript.PRs {
		if (a.URL != "" && pr.URL == a.URL) || (a.Number != 0 && pr.Number == a.Number) {
			if state != "" {
				s.transcript.PRs[i].State = state
			}
			return
		}
	}
	if a.URL == "" && a.Number == 0 {
		return
	}
	if state == "" {
		state = PROpen
	}
	s.transcript.PRs = append(s.transcript.PRs, PR{Number: a.Number, URL: a.URL, Repository: repositoryOf(a.URL), State: state})
}

// repositoryOf is the owner/name of a GitHub pull request URL.
func repositoryOf(url string) string {
	const host = "github.com/"
	i := strings.Index(url, host)
	if i < 0 {
		return ""
	}
	rest := url[i+len(host):]
	parts := strings.SplitN(rest, "/", 4)
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

func (s *transcriptScan) applyCost(rec transcriptLineJSON) {
	cost := Cost{TotalUSD: rec.TotalCostUSD, Models: map[string]ModelCost{}, Known: true}
	for model, mu := range rec.ModelUsage {
		cost.Models[model] = mu.modelCost()
	}
	s.transcript.Cost = cost
}

func (s *transcriptScan) applyTitle(rec transcriptLineJSON) {
	var candidate titleRank
	var title string
	switch rec.Type {
	case "custom-title":
		candidate, title = titleCustom, rec.CustomTitle
	case "ai-title":
		candidate, title = titleAI, rec.AITitle
	case "summary":
		candidate, title = titleSummary, rec.Summary
	default:
		return
	}
	if title == "" || candidate < s.rank {
		return
	}
	s.transcript.Title = title
	s.rank = candidate
}

// openTranscript opens a transcript and counts it, for the tests that
// measure how much work a task does. Item 91.
func openTranscript(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err == nil {
		measure.Count(measure.TranscriptsOpened)
	}
	return f, err
}

// countedReader counts the bytes read through it. Item 91.
type countedReader struct{ r io.Reader }

func (c countedReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	measure.Add(measure.BytesRead, int64(n))
	return n, err
}
