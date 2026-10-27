package config

// Values is every value a key with a closed set of values accepts, which
// 1.0.0 promises along with the keys themselves (item 149). A key whose
// value is open, such as an interval, lists only its named values. The
// acceptance test holds each list to the parser that reads it, so the
// list cannot promise a value the parser has stopped accepting.
var Values = map[string][]string{
	KeyLiveliness:   {"still", "calm", "steady", "lively"},
	KeyDetail:       {"full", "plain"},
	KeyDistrictType: {"random", "guess", "residential", "shopping", "office", "industrial"},
	KeyUsageRefresh: {UsageManual},
	KeyScriptedRuns: {ScriptedHidden, ScriptedShown},
	KeyOnClose:      {OnCloseAsk, OnCloseTray},
	KeyLight:        {LightLive, LightNight, LightDay},
}
