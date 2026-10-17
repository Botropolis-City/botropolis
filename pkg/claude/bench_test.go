package claude_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
)

// BenchmarkReadTranscript reads a long transcript of ordinary prompts,
// tool calls and results, a few of them peer messages: what a read
// costs at load (item 89, against item 91's measures).
func BenchmarkReadTranscript(b *testing.B) {
	var lines []string
	for i := 0; i < 5000; i++ {
		ts := fmt.Sprintf("2026-10-17T12:%02d:%02d.000Z", (i/60)%60, i%60)
		lines = append(lines,
			fmt.Sprintf(`{"type":"user","sessionId":"s","cwd":"/p/a","timestamp":%q,"message":{"role":"user","content":"prompt %d"}}`, ts, i),
			fmt.Sprintf(`{"type":"assistant","sessionId":"s","cwd":"/p/a","timestamp":%q,"message":{"id":"m%d","model":"claude-opus-5","stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":20},"content":[{"type":"tool_use","id":"t%d","name":"Read","input":{"file_path":"/p/a/f%d.go"}}]}}`, ts, i, i, i),
			fmt.Sprintf(`{"type":"user","sessionId":"s","cwd":"/p/a","timestamp":%q,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t%d","content":"ok"}]},"toolUseResult":{"type":"text"}}`, ts, i),
		)
		if i%250 == 0 {
			lines = append(lines, fmt.Sprintf(`{"type":"user","sessionId":"s","cwd":"/p/a","timestamp":%q,"turnOrigin":"peer","promptSource":"system","origin":{"kind":"peer","from":"uds:/run/x/42.sock","verifiedPeerPid":42,"name":"other","msg_id":"id%d"},"message":{"role":"user","content":"<cross-session-message from=\"uds:/run/x/42.sock\">hi</cross-session-message>"}}`, ts, i))
		}
	}
	path := filepath.Join(b.TempDir(), "s.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := claude.ReadTranscript(path); err != nil {
			b.Fatal(err)
		}
	}
}
