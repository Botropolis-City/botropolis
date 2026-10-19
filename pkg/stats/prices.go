package stats

import (
	"strings"
	"time"
)

// LongPrompt is the prompt size, in tokens, above which a tiered model
// prices a whole request at its long rates. The queries group messages
// by it.
const LongPrompt = 100_000

// PricesURL is where the rates below come from, and PricesChecked is
// when they were last read against it.
const (
	PricesURL     = "https://platform.claude.com/docs/en/about-claude/pricing"
	PricesChecked = "2026-10-19"
)

// Rate is one model's list prices in dollars per million tokens, in
// force from From. The fast rates are zero where the model has no fast
// mode; the long rates are zero where it has no prompt tier.
type Rate struct {
	Model                                                     string
	From                                                      time.Time
	Input, Write5m, Write1h, Read, Output                     float64
	FastInput, FastOutput                                     float64
	LongAbove                                                 int64
	LongInput, LongWrite5m, LongWrite1h, LongRead, LongOutput float64
}

// Rates is a price table.
type Rates []Rate

// Counters is what a set of messages used, each counter kept apart
// because each is priced apart. WriteUnsplit is cache writes whose line
// did not say how long they were kept.
type Counters struct {
	Input, Output, Write5m, Write1h, WriteUnsplit, Read int64
	WebSearches, WebFetches                             int64
	Messages                                            int
}

// Add sums two sets of counters.
func (c Counters) Add(o Counters) Counters {
	return Counters{
		Input: c.Input + o.Input, Output: c.Output + o.Output,
		Write5m: c.Write5m + o.Write5m, Write1h: c.Write1h + o.Write1h, WriteUnsplit: c.WriteUnsplit + o.WriteUnsplit,
		Read: c.Read + o.Read, WebSearches: c.WebSearches + o.WebSearches, WebFetches: c.WebFetches + o.WebFetches,
		Messages: c.Messages + o.Messages,
	}
}

// Writes is every cache write, whatever its lifetime.
func (c Counters) Writes() int64 { return c.Write5m + c.Write1h + c.WriteUnsplit }

// Priced is counters with what decides their price: the model, its
// speed, the inference geography, the service tier, whether the prompt
// was over a tiered model's threshold, and when.
type Priced struct {
	Model, Speed, Geo, Tier string
	Long                    bool
	At                      time.Time
	Counters                Counters
}

// Cost is dollars at API list prices. Low and High differ only where
// cache writes have an unknown lifetime; Unpriced counts the messages
// left out because nothing prices them.
type Cost struct {
	Low, High float64
	Unpriced  int
}

// Add sums two costs.
func (c Cost) Add(o Cost) Cost {
	return Cost{Low: c.Low + o.Low, High: c.High + o.High, Unpriced: c.Unpriced + o.Unpriced}
}

const webSearchModel = "web-search"

func isDate(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func matches(row, model string) bool {
	if model == row {
		return true
	}
	rest, ok := strings.CutPrefix(model, row+"-")
	return ok && isDate(rest)
}

// Lookup is the row for a model id, with or without its date suffix,
// in force at a moment.
func (rs Rates) Lookup(model string, at time.Time) (Rate, bool) {
	var best Rate
	found := false
	for _, r := range rs {
		if !matches(r.Model, model) || at.Before(r.From) {
			continue
		}
		if !found || r.From.After(best.From) {
			best, found = r, true
		}
	}
	return best, found
}

// Boundaries is every moment a row starts, after the first, so a query
// can split its sums where a price changes.
func (rs Rates) Boundaries() []time.Time {
	var out []time.Time
	for _, r := range rs {
		if !r.From.IsZero() {
			out = append(out, r.From)
		}
	}
	return out
}

// Price is the list price of a set of counters. Unknown models and
// tiers other than standard are unpriced, never guessed.
func (rs Rates) Price(p Priced) Cost {
	c := p.Counters
	rate, ok := rs.Lookup(p.Model, p.At)
	if !ok || (p.Tier != "" && p.Tier != "standard") {
		return Cost{Unpriced: c.Messages}
	}
	input, w5, w1, read, output := rate.Input, rate.Write5m, rate.Write1h, rate.Read, rate.Output
	if p.Long && rate.LongAbove > 0 {
		input, w5, w1, read, output = rate.LongInput, rate.LongWrite5m, rate.LongWrite1h, rate.LongRead, rate.LongOutput
	}
	if p.Speed == "fast" && rate.FastInput > 0 {
		scale := rate.FastInput / input
		input, w5, w1, read, output = rate.FastInput, w5*scale, w1*scale, read*scale, rate.FastOutput
	}
	geo := 1.0
	if p.Geo == "us" {
		geo = 1.1
	}
	per := func(n int64, r float64) float64 { return float64(n) * r / 1_000_000 * geo }
	base := per(c.Input, input) + per(c.Write5m, w5) + per(c.Write1h, w1) + per(c.Read, read) + per(c.Output, output)
	if search, ok := rs.Lookup(webSearchModel, p.At); ok {
		base += float64(c.WebSearches) * search.Input
	}
	return Cost{Low: base + per(c.WriteUnsplit, w5), High: base + per(c.WriteUnsplit, w1)}
}

func rate(model string, input, w5, w1, read, output float64) Rate {
	return Rate{Model: model, Input: input, Write5m: w5, Write1h: w1, Read: read, Output: output}
}

// sonnet55ReadCut is when Sonnet 5.5's cache reads went from $0.20 to
// $0.10 per million tokens, by the API release notes of that day.
var sonnet55ReadCut = time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

func from(r Rate, at time.Time) Rate {
	r.From = at
	return r
}

func fast(r Rate, input, output float64) Rate {
	r.FastInput, r.FastOutput = input, output
	return r
}

// Prices is Anthropic's list prices, from PricesURL as read on
// PricesChecked. A web search is priced per use, as Input on its own
// row. Item 116.
var Prices = Rates{
	rate("claude-fable-5-1", 10, 12.50, 20, 0.25, 50),
	rate("claude-mythos-5-1", 10, 12.50, 20, 0.25, 50),
	rate("claude-fable-5", 10, 12.50, 20, 1, 50),
	rate("claude-mythos-5", 10, 12.50, 20, 1, 50),
	fast(rate("claude-opus-5-5", 4, 5, 8, 0.20, 20), 8, 40),
	fast(rate("claude-opus-5", 5, 6.25, 10, 0.50, 25), 10, 50),
	fast(rate("claude-opus-4-8", 5, 6.25, 10, 0.50, 25), 10, 50),
	rate("claude-opus-4-7", 5, 6.25, 10, 0.50, 25),
	rate("claude-opus-4-6", 5, 6.25, 10, 0.50, 25),
	rate("claude-opus-4-5", 5, 6.25, 10, 0.50, 25),
	rate("claude-opus-4-1", 15, 18.75, 30, 1.50, 75),
	rate("claude-opus-4", 15, 18.75, 30, 1.50, 75),
	rate("claude-sonnet-5-5", 2, 2.50, 4, 0.20, 10),
	from(rate("claude-sonnet-5-5", 2, 2.50, 4, 0.10, 10), sonnet55ReadCut),
	rate("claude-sonnet-5", 2, 2.50, 4, 0.20, 10),
	rate("claude-sonnet-4-6", 3, 3.75, 6, 0.30, 15),
	rate("claude-sonnet-4-5", 3, 3.75, 6, 0.30, 15),
	rate("claude-sonnet-4", 3, 3.75, 6, 0.30, 15),
	{Model: "claude-haiku-5-5", Input: 0.10, Write5m: 0.125, Write1h: 0.20, Read: 0.01, Output: 0.50,
		LongAbove: LongPrompt, LongInput: 0.50, LongWrite5m: 0.625, LongWrite1h: 1, LongRead: 0.05, LongOutput: 2.50},
	rate("claude-haiku-4-5", 1, 1.25, 2, 0.10, 5),
	rate("claude-3-5-haiku", 0.80, 1, 1.60, 0.08, 4),
	{Model: webSearchModel, Input: 0.01},
}
