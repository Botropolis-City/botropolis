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
	KeyProjection    = "projection"
	KeyRenderScale   = "render_scale"
	KeyReducedMotion = "reduced_motion"
	KeyDailyBudget   = "daily_budget_usd"
	KeyDetail        = "detail"
	KeyUsageRefresh  = "usage_refresh"
	KeyGroupLines    = "group_lines"

	DefaultShutteredDays = 7
	DefaultOvergrownDays = 30
)

const DefaultProjection = "iso"

// DefaultDetail draws the whole city; see city.Detail for why the
// scenery is the part that can go.
const DefaultDetail = "full"

// UsageManual is the usage_refresh value that leaves the probe to the
// u key. It is the default because the probe is a network call. Item 79.
const UsageManual = "manual"

// ParseUsageRefresh reads usage_refresh: manual, or how often the city
// may ask Claude Code to fetch the figures. Zero means manual.
func ParseUsageRefresh(value string) (time.Duration, error) {
	switch value {
	case UsageManual, "", "0":
		return 0, nil
	}
	every, err := time.ParseDuration(value)
	if err != nil || every <= 0 {
		return 0, fmt.Errorf("unknown usage_refresh %q: use manual or an interval such as 30m", value)
	}
	return every, nil
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
	Projection    string
	RenderScale   float64
	ReducedMotion bool
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
	File       string
	// GroupLines draws each group as a line across its districts (item 87).
	GroupLines bool
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
	v.SetDefault(KeyProjection, DefaultProjection)
	v.SetDefault(KeyRenderScale, 0.0)
	v.SetDefault(KeyReducedMotion, false)
	v.SetDefault(KeyDailyBudget, 0.0)
	v.SetDefault(KeyDetail, DefaultDetail)
	v.SetDefault(KeyUsageRefresh, UsageManual)
	v.SetDefault(KeyFind, DefaultFind)
	v.SetDefault(KeyFindCopy, DefaultFindCopy)
	v.SetDefault(KeyFindShow, DefaultFindShow)
	v.SetDefault(KeyNewSession, DefaultNewSession)
	v.SetDefault(KeyExplain, DefaultExplain)
	v.SetDefault(KeyGroupLines, true)
	v.SetConfigName(configName)
	v.AddConfigPath(filepath.Join(configHome(), appDir))
	return v
}

func BindFlags(v *viper.Viper, flags *pflag.FlagSet) {
	flags.String(KeyHome, "", "home directory holding .claude (default: $HOME)")
	flags.String(KeySocket, "", "daemon socket (default: "+proto.SocketPath()+")")
	flags.Int(KeyShutteredDays, DefaultShutteredDays, "how many quiet days before a stopped session is shuttered")
	flags.Int(KeyOvergrownDays, DefaultOvergrownDays, "how many quiet days before a stopped session is overgrown")
	flags.String(KeyProjection, DefaultProjection, "how the city is drawn: iso or top")
	flags.Float64(KeyRenderScale, 0, "chrome and pixel scale (default: follow the display)")
	flags.Bool(KeyReducedMotion, false, "stop every animation and keep the colours")
	flags.Float64(KeyDailyBudget, 0, "a daily spend to measure the strip and the plant against, in USD (0 for none)")
	flags.String(KeyDetail, DefaultDetail, "how much of the city to draw: full or plain (plain drops the scenery)")
	flags.String(KeyUsageRefresh, UsageManual, "how often to fetch plan usage: manual (press u) or an interval such as 30m")
	flags.Bool(KeyGroupLines, true, "draw each group of sessions as a line across its districts")
	_ = v.BindPFlag(KeyHome, flags.Lookup(KeyHome))
	_ = v.BindPFlag(KeySocket, flags.Lookup(KeySocket))
	_ = v.BindPFlag(KeyShutteredDays, flags.Lookup(KeyShutteredDays))
	_ = v.BindPFlag(KeyOvergrownDays, flags.Lookup(KeyOvergrownDays))
	_ = v.BindPFlag(KeyProjection, flags.Lookup(KeyProjection))
	_ = v.BindPFlag(KeyRenderScale, flags.Lookup(KeyRenderScale))
	_ = v.BindPFlag(KeyReducedMotion, flags.Lookup(KeyReducedMotion))
	_ = v.BindPFlag(KeyDailyBudget, flags.Lookup(KeyDailyBudget))
	_ = v.BindPFlag(KeyDetail, flags.Lookup(KeyDetail))
	_ = v.BindPFlag(KeyUsageRefresh, flags.Lookup(KeyUsageRefresh))
	flags.String(KeyFind, DefaultFind, "the chord that opens the finder; mod is ⌘ on macOS and Ctrl elsewhere")
	flags.String(KeyFindCopy, DefaultFindCopy, "the finder's chord that copies how to resume a session")
	flags.String(KeyFindShow, DefaultFindShow, "the finder's chord that shows a session on the map")
	flags.String(KeyNewSession, DefaultNewSession, "the chord that opens New session, which starts a session in any directory")
	flags.String(KeyExplain, DefaultExplain, "the key held to caption everything on the map")
	_ = v.BindPFlag(KeyFind, flags.Lookup(KeyFind))
	_ = v.BindPFlag(KeyFindCopy, flags.Lookup(KeyFindCopy))
	_ = v.BindPFlag(KeyFindShow, flags.Lookup(KeyFindShow))
	_ = v.BindPFlag(KeyNewSession, flags.Lookup(KeyNewSession))
	_ = v.BindPFlag(KeyExplain, flags.Lookup(KeyExplain))
	_ = v.BindPFlag(KeyGroupLines, flags.Lookup(KeyGroupLines))
}

func New(v *viper.Viper) (*Config, error) {
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, err
		}
	}
	cfg := &Config{
		Home:          v.GetString(KeyHome),
		HomeSet:       v.GetString(KeyHome) != "",
		Socket:        v.GetString(KeySocket),
		SocketSet:     v.GetString(KeySocket) != "",
		Terminal:      v.GetString(KeyTerminal),
		HookCommand:   v.GetString(KeyHookCommand),
		ShutteredDays: v.GetInt(KeyShutteredDays),
		OvergrownDays: v.GetInt(KeyOvergrownDays),
		CodexHome:     v.GetString(KeyCodexHome),
		Projection:    v.GetString(KeyProjection),
		RenderScale:   v.GetFloat64(KeyRenderScale),
		ReducedMotion: v.GetBool(KeyReducedMotion),
		DailyBudget:   v.GetFloat64(KeyDailyBudget),
		Detail:        v.GetString(KeyDetail),
		UsageRefresh:  v.GetString(KeyUsageRefresh),
		FindKey:       v.GetString(KeyFind),
		FindCopyKey:   v.GetString(KeyFindCopy),
		FindShowKey:   v.GetString(KeyFindShow),
		NewSessionKey: v.GetString(KeyNewSession),
		ExplainKey:    v.GetString(KeyExplain),
		GroupLines:    v.GetBool(KeyGroupLines),
		File:          v.ConfigFileUsed(),
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
