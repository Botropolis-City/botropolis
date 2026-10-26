package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"botropolis.city/botropolis/pkg/proto"
)

const (
	envPrefix  = "BOTROPOLIS"
	appDir     = "botropolis"
	configName = "config"

	KeyHome          = "home"
	KeySocket        = "socket"
	KeyTerminal      = "terminal"
	KeyHookCommand   = "hook_command"
	KeyShutteredDays = "shuttered_days"
	KeyOvergrownDays = "overgrown_days"
	KeyCodexHome     = "codex_home"
	KeyRenderScale   = "render_scale"
	KeyReducedMotion = "reduced_motion"
	// KeyLiveliness is how much the city moves, still to lively; reduced
	// motion is still whatever it says. Item 89.
	KeyLiveliness   = "liveliness"
	KeyDailyBudget  = "daily_budget_usd"
	KeyDetail       = "detail"
	KeyUsageRefresh = "usage_refresh"
	KeyGroupLines   = "group_lines"
	// KeyLabels shows or hides every label on the map; signs stay.
	// Item 102.
	KeyLabels = "labels"
	// LabelsShown and LabelsHidden are the labels row's two values.
	LabelsShown  = "shown"
	LabelsHidden = "hidden"
	// KeyDistrictType is how a new district's type is chosen: random,
	// guess, or one type for all. Item 109.
	KeyDistrictType = "district_type"
	// KeyStats keeps every assistant message's usage in the statistics
	// store; on by default. Item 110.
	KeyStats = "stats"
	// KeyGoneSessions keeps sessions whose transcript Claude Code deleted
	// on the map; off by default. Item 274.
	KeyGoneSessions = "gone_sessions"
	// KeyGlobalShortcut is the chord that opens the finder from any
	// application, empty for off: there is no default. Item 84.
	KeyGlobalShortcut = "global_shortcut"
	// KeyConfig names a config file to use instead of the default one,
	// so a frame can run on settings of its own.
	KeyConfig = "config"

	DefaultShutteredDays = 7
	// KeyJournalHours is how long away the map must have been for the
	// journal to open by itself (item 86).
	KeyJournalHours      = "journal_hours"
	DefaultJournalHours  = 8
	DefaultOvergrownDays = 30
)

// DefaultDistrictType gives each new district a type hashed from its
// path. Item 109.
const DefaultDistrictType = "random"

// DefaultLiveliness is between calm and lively, and Stillness the level
// reduced motion asks for. Item 89.
const (
	DefaultLiveliness = "steady"
	Stillness         = "still"
)

// DefaultDetail draws the whole city; see city.Detail for why the
// scenery is the part that can go.
const DefaultDetail = "full"

// UsageManual is the usage_refresh value that leaves the probe to the
// u key. It is the default because the probe is a network call. Item 79.
const UsageManual = "manual"

// UsageRefreshFloor is the shortest interval the city probes on. Item
// 160: each probe is a Claude Code of its own, measured at 25–45 s and
// 200–420 MiB, and a background helper may run for a tenth of the time
// at most, so 5m was over budget. Aria, 2026-10-21: "Probe less often."
const UsageRefreshFloor = 15 * time.Minute

// ParseUsageRefresh reads usage_refresh: manual, or how often the city
// may ask Claude Code to fetch the figures. Zero means manual. An
// interval under the floor is raised to it rather than refused, so a
// config written for an earlier version still reads.
func ParseUsageRefresh(value string) (time.Duration, error) {
	switch value {
	case UsageManual, "", "0":
		return 0, nil
	}
	every, err := time.ParseDuration(value)
	if err != nil || every <= 0 {
		return 0, fmt.Errorf("unknown usage_refresh %q: use manual or an interval such as 30m", value)
	}
	return max(every, UsageRefreshFloor), nil
}

type Config struct {
	Home          string
	HomeSet       bool
	Socket        string
	SocketSet     bool
	Terminal      string
	HookCommand   string
	ShutteredDays int
	OvergrownDays int
	CodexHome     string
	RenderScale   float64
	ReducedMotion bool
	Liveliness    string
	DistrictType  string
	Stats         bool
	DailyBudget   float64
	Detail        string
	UsageRefresh  string
	// FindKey, FindCopyKey and FindShowKey are the finder's chords, as
	// set; FindChords reads them. Item 82.
	FindKey     string
	FindCopyKey string
	FindShowKey string
	// NewSessionKey is the chord that opens New session (item 87).
	NewSessionKey string
	// ExplainKey is held for explain mode (item 88).
	ExplainKey string
	// TriageKey starts triage (item 88).
	TriageKey string
	// JournalKey brings the journal back, and JournalHours is how long
	// away the map must have been for the journal to open by itself
	// (item 86).
	JournalKey   string
	JournalHours int
	// GlobalShortcut opens the finder from any application; empty is off
	// (item 84).
	GlobalShortcut string
	// StreamKey hides and shows the stream (item 88).
	StreamKey string
	// CityPlanKey opens and closes the City Plan (item 106).
	CityPlanKey string
	// LabelsKey turns every label off and on (item 102).
	LabelsKey string
	File      string
	// GroupLines draws each group as a line across its districts (item 87).
	GroupLines bool
	// Labels shows the map's labels; signs on posts stay either way
	// (item 102).
	Labels bool
	// GoneSessions keeps sessions whose transcript is gone on the map
	// (item 274).
	GoneSessions bool
}

func NewViper() *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()
	v.SetDefault(KeyHome, "")
	v.SetDefault(KeySocket, "")
	v.SetDefault(KeyTerminal, "")
	v.SetDefault(KeyHookCommand, "botropolis-hook")
	v.SetDefault(KeyShutteredDays, DefaultShutteredDays)
	v.SetDefault(KeyOvergrownDays, DefaultOvergrownDays)
	v.SetDefault(KeyCodexHome, "")
	v.SetDefault(KeyRenderScale, 0.0)
	v.SetDefault(KeyReducedMotion, false)
	v.SetDefault(KeyLiveliness, DefaultLiveliness)
	v.SetDefault(KeyDistrictType, DefaultDistrictType)
	v.SetDefault(KeyStats, true)
	v.SetDefault(KeyDailyBudget, 0.0)
	v.SetDefault(KeyDetail, DefaultDetail)
	v.SetDefault(KeyUsageRefresh, UsageManual)
	v.SetDefault(KeyFind, DefaultFind)
	v.SetDefault(KeyFindCopy, DefaultFindCopy)
	v.SetDefault(KeyFindShow, DefaultFindShow)
	v.SetDefault(KeyNewSession, DefaultNewSession)
	v.SetDefault(KeyExplain, DefaultExplain)
	v.SetDefault(KeyTriage, DefaultTriage)
	v.SetDefault(KeyJournal, DefaultJournal)
	v.SetDefault(KeyJournalHours, DefaultJournalHours)
	v.SetDefault(KeyStream, DefaultStream)
	v.SetDefault(KeyCityPlan, DefaultCityPlan)
	v.SetDefault(KeyGroupLines, true)
	v.SetDefault(KeyLabels, true)
	v.SetDefault(KeyGoneSessions, false)
	v.SetDefault(KeyLabelsToggle, DefaultLabelsToggle)
	v.SetDefault(KeyGlobalShortcut, "")
	v.SetConfigName(configName)
	v.AddConfigPath(filepath.Join(configHome(), appDir))
	return v
}

func BindFlags(v *viper.Viper, flags *pflag.FlagSet) {
	flags.String(KeyHome, "", "the home `dir` holding .claude (default: $HOME)")
	flags.String(KeySocket, "", "the daemon's socket `path` (default: "+proto.SocketPath()+")")
	flags.Int(KeyShutteredDays, DefaultShutteredDays, "how many quiet days before a stopped session is shuttered")
	flags.Int(KeyOvergrownDays, DefaultOvergrownDays, "how many quiet days before a stopped session is overgrown")
	flags.Float64(KeyRenderScale, 0, "chrome and pixel scale (default: follow the display)")
	flags.Bool(KeyReducedMotion, false, "stop every animation and keep the colours: liveliness still")
	flags.String(KeyLiveliness, DefaultLiveliness, "how much the city moves: still, calm, steady or lively")
	flags.String(KeyDistrictType, DefaultDistrictType, "how a new district's type is chosen: random, guess (from its file names), or one of residential, shopping, office and industrial for all")
	flags.Bool(KeyStats, true, "keep each assistant message's usage in a statistics database of Botropolis's own")
	flags.Float64(KeyDailyBudget, 0, "a daily spend to measure the strip and the plant against, in USD (0 for none)")
	flags.String(KeyDetail, DefaultDetail, "how much of the city to draw: full or plain (plain drops the scenery)")
	flags.String(KeyUsageRefresh, UsageManual, "how often to fetch plan usage: manual (press u) or an interval such as 30m; anything under 15m means 15m")
	flags.Bool(KeyGroupLines, true, "draw each group of sessions as a line across its districts")
	flags.Bool(KeyLabels, true, "draw the map's labels: group badges, line names, street names and plates; signs on posts stay either way")
	flags.String(KeyConfig, "", "the config `file` to read and save (default: "+DefaultFile()+")")
	_ = v.BindPFlag(KeyConfig, flags.Lookup(KeyConfig))
	_ = v.BindPFlag(KeyHome, flags.Lookup(KeyHome))
	_ = v.BindPFlag(KeySocket, flags.Lookup(KeySocket))
	_ = v.BindPFlag(KeyShutteredDays, flags.Lookup(KeyShutteredDays))
	_ = v.BindPFlag(KeyOvergrownDays, flags.Lookup(KeyOvergrownDays))
	_ = v.BindPFlag(KeyRenderScale, flags.Lookup(KeyRenderScale))
	_ = v.BindPFlag(KeyReducedMotion, flags.Lookup(KeyReducedMotion))
	_ = v.BindPFlag(KeyLiveliness, flags.Lookup(KeyLiveliness))
	_ = v.BindPFlag(KeyDistrictType, flags.Lookup(KeyDistrictType))
	_ = v.BindPFlag(KeyStats, flags.Lookup(KeyStats))
	_ = v.BindPFlag(KeyDailyBudget, flags.Lookup(KeyDailyBudget))
	_ = v.BindPFlag(KeyDetail, flags.Lookup(KeyDetail))
	_ = v.BindPFlag(KeyUsageRefresh, flags.Lookup(KeyUsageRefresh))
	flags.String(KeyFind, DefaultFind, "the chord that opens the finder; mod is ⌘ on macOS and Ctrl elsewhere")
	flags.String(KeyFindCopy, DefaultFindCopy, "the finder's chord that copies how to resume a session")
	flags.String(KeyFindShow, DefaultFindShow, "the finder's chord that shows a session on the map")
	flags.String(KeyNewSession, DefaultNewSession, "the chord that opens New session, which starts a session in any directory")
	flags.String(KeyExplain, DefaultExplain, "the key held to caption everything on the map")
	flags.String(KeyTriage, DefaultTriage, "the key that walks what waits on you, longest-waiting first")
	flags.String(KeyJournal, DefaultJournal, "the key that brings back the journal of where you were")
	flags.Int(KeyJournalHours, DefaultJournalHours, "how many hours away before the journal opens by itself")
	flags.String(KeyStream, DefaultStream, "the key that hides and shows the stream down the right")
	flags.String(KeyCityPlan, DefaultCityPlan, "the key that opens and closes the City Plan, a flat map of every district")
	flags.String(KeyLabelsToggle, DefaultLabelsToggle, "the key that turns every label on the map off and on")
	_ = v.BindPFlag(KeyFind, flags.Lookup(KeyFind))
	_ = v.BindPFlag(KeyFindCopy, flags.Lookup(KeyFindCopy))
	_ = v.BindPFlag(KeyFindShow, flags.Lookup(KeyFindShow))
	_ = v.BindPFlag(KeyNewSession, flags.Lookup(KeyNewSession))
	_ = v.BindPFlag(KeyExplain, flags.Lookup(KeyExplain))
	_ = v.BindPFlag(KeyTriage, flags.Lookup(KeyTriage))
	_ = v.BindPFlag(KeyJournal, flags.Lookup(KeyJournal))
	_ = v.BindPFlag(KeyJournalHours, flags.Lookup(KeyJournalHours))
	_ = v.BindPFlag(KeyStream, flags.Lookup(KeyStream))
	_ = v.BindPFlag(KeyCityPlan, flags.Lookup(KeyCityPlan))
	_ = v.BindPFlag(KeyGroupLines, flags.Lookup(KeyGroupLines))
	_ = v.BindPFlag(KeyLabels, flags.Lookup(KeyLabels))
	_ = v.BindPFlag(KeyLabelsToggle, flags.Lookup(KeyLabelsToggle))
	for _, key := range deprecatedKeyFlags {
		if f := flags.Lookup(key); f != nil {
			f.Hidden = true
		}
	}
	flags.Var(&setValue{v: v}, KeySet, "set a config key for this run; repeatable (see botropolis-config(5))")
}

func New(v *viper.Viper) (*Config, error) {
	file := v.GetString(KeyConfig)
	if file != "" {
		v.SetConfigFile(file)
	}
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		missing := file != "" && errors.Is(err, os.ErrNotExist)
		if !errors.As(err, &notFound) && !missing {
			return nil, err
		}
	}
	cfg := &Config{
		Home:           v.GetString(KeyHome),
		HomeSet:        v.GetString(KeyHome) != "",
		Socket:         v.GetString(KeySocket),
		SocketSet:      v.GetString(KeySocket) != "",
		Terminal:       v.GetString(KeyTerminal),
		HookCommand:    v.GetString(KeyHookCommand),
		ShutteredDays:  v.GetInt(KeyShutteredDays),
		OvergrownDays:  v.GetInt(KeyOvergrownDays),
		CodexHome:      v.GetString(KeyCodexHome),
		RenderScale:    v.GetFloat64(KeyRenderScale),
		ReducedMotion:  v.GetBool(KeyReducedMotion),
		Liveliness:     v.GetString(KeyLiveliness),
		DistrictType:   v.GetString(KeyDistrictType),
		Stats:          v.GetBool(KeyStats),
		DailyBudget:    v.GetFloat64(KeyDailyBudget),
		Detail:         v.GetString(KeyDetail),
		UsageRefresh:   v.GetString(KeyUsageRefresh),
		FindKey:        v.GetString(KeyFind),
		FindCopyKey:    v.GetString(KeyFindCopy),
		FindShowKey:    v.GetString(KeyFindShow),
		NewSessionKey:  v.GetString(KeyNewSession),
		ExplainKey:     v.GetString(KeyExplain),
		TriageKey:      v.GetString(KeyTriage),
		JournalKey:     v.GetString(KeyJournal),
		JournalHours:   v.GetInt(KeyJournalHours),
		StreamKey:      v.GetString(KeyStream),
		CityPlanKey:    v.GetString(KeyCityPlan),
		LabelsKey:      v.GetString(KeyLabelsToggle),
		GroupLines:     v.GetBool(KeyGroupLines),
		Labels:         v.GetBool(KeyLabels),
		GoneSessions:   v.GetBool(KeyGoneSessions),
		GlobalShortcut: v.GetString(KeyGlobalShortcut),
		File:           v.ConfigFileUsed(),
	}
	if cfg.ReducedMotion {
		cfg.Liveliness = Stillness
	}
	if _, err := ParseUsageRefresh(cfg.UsageRefresh); err != nil {
		return nil, err
	}
	if err := checkLadder(cfg.ShutteredDays, cfg.OvergrownDays); err != nil {
		return nil, err
	}
	if cfg.Home == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		cfg.Home = home
	}
	if cfg.Socket == "" {
		cfg.Socket = proto.SocketPath()
	}
	if cfg.CodexHome == "" {
		cfg.CodexHome = filepath.Join(cfg.Home, ".codex")
	}
	return cfg, nil
}

// checkLadder refuses thresholds that would overgrow a session before
// shuttering it. Item 86.
func checkLadder(shuttered, overgrown int) error {
	if overgrown <= shuttered {
		return fmt.Errorf("%s (%d) must be more than %s (%d)", KeyOvergrownDays, overgrown, KeyShutteredDays, shuttered)
	}
	return nil
}

// DefaultFile is where settings are written when no config file was read.
func DefaultFile() string {
	return filepath.Join(configHome(), appDir, configName+".toml")
}

// Save writes one key into the config file at path (or the default file
// when path is empty), keeping whatever else the file holds and never
// spelling out defaults.
func Save(path, key string, value any) error {
	if path == "" {
		path = DefaultFile()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw := viper.New()
	raw.SetConfigFile(path)
	if err := raw.ReadInConfig(); err != nil {
		if _, missing := err.(*os.PathError); !missing && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	raw.Set(key, value)
	return raw.WriteConfigAs(path)
}

func configHome() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config")
}
