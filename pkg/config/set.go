package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// KeySet is the flag that sets any config key for one run, in place of a
// flag per key. Item 167.
const KeySet = "set"

// deprecatedKeyFlags are the keys that had a flag of their own before
// --set. They keep working, hidden, until 1.0.0.
var deprecatedKeyFlags = []string{
	KeyShutteredDays, KeyOvergrownDays, KeyRenderScale, KeyReducedMotion,
	KeyLiveliness, KeyDistrictType, KeyStats, KeyDailyBudget, KeyDetail,
	KeyUsageRefresh, KeyGroupLines, KeyLabels,
	KeyFind, KeyFindCopy, KeyFindShow, KeyNewSession, KeyExplain, KeyTriage,
	KeyJournal, KeyJournalHours, KeyStream, KeyCityPlan, KeyLabelsToggle,
}

// setValue applies each key=value as viper's override, which outranks
// the file, the environment and every flag.
type setValue struct {
	v    *viper.Viper
	sets []string
}

func (s *setValue) Set(raw string) error {
	key, value, ok := strings.Cut(raw, "=")
	if !ok {
		return fmt.Errorf("%q is not key=value", raw)
	}
	if !slices.Contains(Keys(s.v), key) {
		return fmt.Errorf("unknown key %q: see botropolis-config(5)", key)
	}
	s.v.Set(key, value)
	s.sets = append(s.sets, raw)
	return nil
}

func (s *setValue) String() string { return strings.Join(s.sets, ",") }

func (s *setValue) Type() string { return "key=value" }

// Keys are the config file's keys, sorted.
func Keys(v *viper.Viper) []string {
	var keys []string
	for _, key := range v.AllKeys() {
		if key != KeyConfig && key != KeySet {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// DeprecationNotes are a line for each old per-key flag this run used,
// naming the --set that replaces it.
func DeprecationNotes(flags *pflag.FlagSet) []string {
	var notes []string
	for _, key := range deprecatedKeyFlags {
		if f := flags.Lookup(key); f != nil && f.Changed {
			notes = append(notes, fmt.Sprintf("--%s is deprecated and goes at 1.0: use --set %s=%s", key, key, f.Value.String()))
		}
	}
	return notes
}
