package ui

import "botropolis.city/botropolis/pkg/stats"

// Headline, CountersLine and PausedLine are the one wording of usage,
// from pkg/stats, so pkg/city's cards can use it too. Item 116.
func Headline(c stats.Cost, messages int) string { return stats.Headline(c, messages) }

// CountersLine is stats.CountersLine.
func CountersLine(c stats.Counters) string { return stats.CountersLine(c) }

// PausedLine is stats.PausedLine.
func PausedLine(reason string) string { return stats.PausedLine(reason) }
