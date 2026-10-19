package acceptance_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/harness/codex"
)

func syntheticRollout(n, size int) string {
	var b strings.Builder
	ts := func(i int) string {
		return time.Date(2026, 10, 16, 7, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)
	}
	fmt.Fprintf(&b, `{"timestamp":%q,"type":"session_meta","payload":{"id":"019a3c2e-0000-7c3d-9e4f-%012d","timestamp":%q,"cwd":"/home/u/p/%d","originator":"codex_cli_rs","cli_version":"0.42.0","instructions":%q,"git":{"branch":"main","commit_hash":"abc"}}}`+"\n", ts(0), n, ts(0), n%20, strings.Repeat("Be careful. ", 400))
	fmt.Fprintf(&b, `{"timestamp":%q,"type":"turn_context","payload":{"cwd":"/home/u/p","model":"gpt-5-codex","effort":"medium"}}`+"\n", ts(1))
	output := strings.Repeat(`line of shell output with \"quotes\" and \\ paths\n`, 60)
	reasoning := strings.Repeat("QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo=", 50)
	for i := 2; b.Len() < size; i += 6 {
		fmt.Fprintf(&b, `{"timestamp":%q,"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"step %d of the work"}]}}`+"\n", ts(i), i)
		fmt.Fprintf(&b, `{"timestamp":%q,"type":"response_item","payload":{"type":"reasoning","summary":[],"content":null,"encrypted_content":%q}}`+"\n", ts(i+1), reasoning)
		fmt.Fprintf(&b, `{"timestamp":%q,"type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"{\"command\":[\"ls\"]}","call_id":"c%d"}}`+"\n", ts(i+2), i)
		fmt.Fprintf(&b, `{"timestamp":%q,"type":"response_item","payload":{"type":"function_call_output","call_id":"c%d","output":"%s"}}`+"\n", ts(i+3), i, output)
		fmt.Fprintf(&b, `{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"output_tokens":%d,"reasoning_output_tokens":10,"total_tokens":1},"last_token_usage":{"input_tokens":7000,"cached_input_tokens":6000,"output_tokens":400},"model_context_window":272000}}}`+"\n", ts(i+4), 1000*i, 800*i, 50*i)
		fmt.Fprintf(&b, `{"timestamp":%q,"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Done with step %d."}]}}`+"\n", ts(i+5), i)
	}
	return b.String()
}

// BenchmarkLoadAHeavyHistory is the first load of a heavy Codex history:
// 2,000 rollouts of about 100 KB, each mostly tool output and reasoning,
// as item 86 measured it at 3.2 s. Item 86's own rollouts were never kept,
// so this rebuilds them, and is the measure item 93 is judged by.
func BenchmarkLoadAHeavyHistory(b *testing.B) {
	home := b.TempDir()
	dir := filepath.Join(home, "sessions", "2026", "10", "16")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		b.Fatal(err)
	}
	for n := 0; n < 2000; n++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("rollout-%04d.jsonl", n)), []byte(syntheticRollout(n, 100<<10)), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	at := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snapshot, err := codex.NewLoader(home).Load(at)
		if err != nil || len(snapshot.Sessions) != 2000 {
			b.Fatalf("loaded %d sessions: %v", len(snapshot.Sessions), err)
		}
	}
}
