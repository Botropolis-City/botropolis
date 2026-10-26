package group

import (
	"slices"
	"strings"
)

// commonKinds are the kinds of key a pair can share as evidence: a key
// more sessions share than a group can hold says nothing about which of
// them work together, as a word too many sessions use says nothing
// (item 271).
var commonKinds = map[int]string{refKey: "reference", teamKey: "team", branchKey: "branch", fileKey: "file"}

// commonKeys are the keys of commonKinds that more than MaxMembers
// members hold.
func (d *detection) commonKeys() map[indexKey]bool {
	counts := map[indexKey]int{}
	seen := map[indexKey]bool{}
	for i, m := range d.members {
		clear(seen)
		count := func(kind int, key string) {
			k := indexKey{kind, key}
			if !seen[k] {
				seen[k] = true
				counts[k]++
			}
		}
		for _, r := range d.refs[i] {
			count(refKey, r)
		}
		if m.Team != "" {
			count(teamKey, m.Team)
		}
		for _, b := range branchKeys(m.Branch) {
			count(branchKey, b)
		}
		for _, f := range m.Files {
			count(fileKey, f)
		}
	}
	common := map[indexKey]bool{}
	for k, n := range counts {
		if n > MaxMembers {
			common[k] = true
		}
	}
	return common
}

// isCommon is whether key, of kind, is too common to be evidence.
func (d *detection) isCommon(kind int, key string) bool {
	return len(d.common) > 0 && d.common[indexKey{kind, key}]
}

// uncommon are the keys of xs that are not too common to be evidence.
func (d *detection) uncommon(kind int, xs []string) []string {
	if len(d.common) == 0 {
		return xs
	}
	return slices.DeleteFunc(slices.Clone(xs), func(x string) bool { return d.isCommon(kind, x) })
}

// commonNames are the common keys as Result.Common gives them.
func (d *detection) commonNames() []string {
	if len(d.common) == 0 {
		return nil
	}
	set := map[string]bool{}
	for k := range d.common {
		name := k.key
		kind := commonKinds[k.kind]
		if k.kind == branchKey {
			if ref, ok := strings.CutPrefix(name, "ref:"); ok {
				name, kind = ref, commonKinds[refKey]
			} else {
				name = strings.TrimPrefix(name, "branch:")
			}
		}
		set[kind+" "+name] = true
	}
	return sortedKeys(set)
}
