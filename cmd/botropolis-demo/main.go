package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/demo"
	"botropolis.city/botropolis/pkg/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	root := &cobra.Command{
		Use:           "botropolis-demo",
		Short:         "Stage a demo city from the recorded corpus, so no real session is ever filmed",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.AddCommand(stageCommand(), unstageCommand(), filmCommand(), recordCommand(), mcpCommand(), scrubCommand(), retitleCommand())
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(errOut, "botropolis-demo: %v\n", err)
		return 1
	}
	return 0
}

func stageCommand() *cobra.Command {
	var corpus, scenarioPath, home string
	var hold time.Duration
	var play bool
	cmd := &cobra.Command{
		Use:   "stage",
		Short: "Write a home for a scenario and start stand-ins for its live sessions",
		Long: "Writes a Claude home for the scenario into --home: each placed session's\n" +
			"transcript moved to end relative to now, a session record for every one\n" +
			"that is not parked, pointed at a detached sleep, and the MCP config.\n" +
			"Point botropolis at it with --home and --socket, and run unstage after.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			scenario, err := demo.LoadScenario(scenarioPath)
			if err != nil {
				return err
			}
			if scenario.Name == "" {
				scenario.Name = filepath.Base(scenarioPath)
			}
			if err := demo.Unstage(home); err != nil {
				return err
			}
			if err := os.RemoveAll(filepath.Join(home, ".claude")); err != nil {
				return err
			}
			if err := os.MkdirAll(home, 0o700); err != nil {
				return err
			}
			start := time.Now()
			director, err := demo.NewDirector(demo.Corpus{Dir: corpus}, scenario, home, start, demo.Sleepers(home, hold))
			if err != nil {
				_ = demo.Unstage(home)
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "staged %s: %d sessions in %s\n", scenario.Name, len(scenario.Sessions), home)
			if !play {
				return nil
			}
			for _, e := range director.Events() {
				fmt.Fprintf(cmd.OutOrStdout(), "  at %s: session %d\n", e.At, e.Index)
			}
			return director.Play(cmd.Context(), start)
		},
	}
	cmd.Flags().StringVar(&corpus, "corpus", "demo/corpus", "directory of recorded transcripts")
	cmd.Flags().StringVar(&scenarioPath, "scenario", "", "scenario file")
	cmd.Flags().StringVar(&home, "home", "", "directory to stage the home into")
	cmd.Flags().DurationVar(&hold, "hold", time.Hour, "how long a stand-in lives if never unstaged")
	cmd.Flags().BoolVar(&play, "play", false, "then play the scenario's timeline against the home, in real time")
	_ = cmd.MarkFlagRequired("scenario")
	_ = cmd.MarkFlagRequired("home")
	return cmd
}

func unstageCommand() *cobra.Command {
	var home string
	cmd := &cobra.Command{
		Use:   "unstage",
		Short: "End a staged home's stand-ins",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return demo.Unstage(home)
		},
	}
	cmd.Flags().StringVar(&home, "home", "", "the staged home")
	_ = cmd.MarkFlagRequired("home")
	return cmd
}

func filmCommand() *cobra.Command {
	var corpus, out, botropolis, ffmpeg string
	var fresh, adopt bool
	var source string
	cmd := &cobra.Command{
		Use:   "film SCENARIO...",
		Short: "Make every shot of each scenario and write a manifest",
		Long: "Stages each scenario afresh for every shot, runs the city headless\n" +
			"against it with HOME and the XDG directories in a scratch directory,\n" +
			"encodes clips with ffmpeg, and writes OUT/manifest.json. A shot made\n" +
			"from the same scenario, corpus and binaries as last time is reused;\n" +
			"--fresh films everything.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, paths []string) error {
			if botropolis == "" {
				self, err := os.Executable()
				if err != nil {
					return err
				}
				botropolis = filepath.Join(filepath.Dir(self), "botropolis")
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			logPath := filepath.Join(out, "film.log")
			log, err := os.Create(logPath)
			if err != nil {
				return err
			}
			defer func() { _ = log.Close() }()
			progress := demo.NewProgress(os.Stderr, 0)
			stop := progress.Ticking()
			defer stop()
			self, err := os.Executable()
			if err != nil {
				return err
			}
			f := demo.Filmer{
				Corpus: demo.Corpus{Dir: corpus}, Botropolis: botropolis, FFmpeg: ffmpeg,
				Out: out, Version: version.Version, Log: log, Now: time.Now, Progress: progress,
				Self: self, Fresh: fresh, Source: source, Adopt: adopt,
			}
			if _, err := f.Film(paths); err != nil {
				return fmt.Errorf("%w (the city's own output is in %s)", err, logPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&corpus, "corpus", "demo/corpus", "directory of recorded transcripts")
	cmd.Flags().StringVar(&out, "out", "dist/demo", "directory to write media and manifest.json into")
	cmd.Flags().StringVar(&botropolis, "botropolis", "", "the botropolis binary (default: beside this one)")
	cmd.Flags().StringVar(&ffmpeg, "ffmpeg", "ffmpeg", "the ffmpeg binary")
	cmd.Flags().BoolVar(&fresh, "fresh", false, "film every shot, even those unchanged since the last run")
	cmd.Flags().StringVar(&source, "source", "", "a hash of the source the binaries were built from, so the cache holds across machines")
	cmd.Flags().BoolVar(&adopt, "adopt", false, "file every cached shot under its current key without filming it, when the key changed and the picture cannot have")
	return cmd
}

func mcpCommand() *cobra.Command {
	var data string
	cmd := &cobra.Command{
		Use:       "mcp tracker|weather",
		Short:     "Serve a fictional MCP server on stdio for recorded sessions to call",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"tracker", "weather"},
		RunE: func(cmd *cobra.Command, args []string) error {
			server := demo.Weather()
			switch args[0] {
			case "tracker":
				server = demo.Tracker(data)
			case "weather":
			default:
				return fmt.Errorf("no server %q: want tracker or weather", args[0])
			}
			return demo.ServeMCP(server, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&data, "data", "", "directory of <project>.json issue files, for tracker")
	return cmd
}

func recordCommand() *cobra.Command {
	var scripts, projects, tracker, skills, corpus, home, src, only string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record the scripted sessions with the real claude CLI (run inside the recorder container)",
		Long: "Copies the toy projects into --src as git repositories, installs the\n" +
			"skills and MCP servers, then runs `claude -p` for every scripted prompt\n" +
			"the corpus does not hold yet, and exports each finished session's\n" +
			"transcript and subagents -- and nothing else -- into --corpus.\n" +
			"--dry-run prints what would be recorded and the most it could cost.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := filepath.Glob(filepath.Join(scripts, "*.yaml"))
			if err != nil {
				return err
			}
			sort.Strings(paths)
			var all []demo.Script
			for _, p := range paths {
				s, err := demo.LoadScript(p)
				if err != nil {
					return err
				}
				all = append(all, filterScript(s, only))
			}
			out := cmd.OutOrStdout()
			if dryRun {
				return printPlan(out, all, corpus)
			}
			self, err := os.Executable()
			if err != nil {
				return err
			}
			config, err := demo.Workspace{Projects: projects, Tracker: tracker, Skills: skills, Home: home, Src: src, Self: self}.Prepare()
			if err != nil {
				return err
			}
			prompts := 0
			for _, s := range all {
				for _, session := range s.Sessions {
					if _, err := os.Stat(filepath.Join(corpus, s.Project, demo.SessionID(s.Project, session.Label)+".jsonl")); err != nil {
						prompts += len(session.Prompts)
					}
				}
			}
			progress := demo.NewProgress(os.Stderr, prompts)
			stop := progress.Ticking()
			defer stop()
			log := cmd.ErrOrStderr()
			if progress.TTY {
				log = io.Discard
			}
			r := demo.Recorder{Home: home, Src: src, Corpus: corpus, MCPConfig: config, Claude: runClaude, Log: log, Progress: progress}
			var spent []demo.Prompted
			var failed error
			for _, s := range all {
				log, err := r.Record(cmd.Context(), s)
				spent = append(spent, log...)
				if err != nil {
					failed = err
					break
				}
			}
			var total float64
			for _, p := range spent {
				total += p.CostUSD
			}
			progress.Finish(fmt.Sprintf("recorded %d prompts for $%.2f", len(spent), total))
			data, err := json.MarshalIndent(spent, "", "  ")
			if err == nil {
				_ = os.WriteFile(filepath.Join(corpus, "recording-"+time.Now().UTC().Format("20060102T150405Z")+".json"), append(data, '\n'), 0o644)
			}
			return failed
		},
	}
	cmd.Flags().StringVar(&scripts, "scripts", "/demo/scripts", "directory of <project>.yaml session scripts")
	cmd.Flags().StringVar(&projects, "projects", "/demo/projects", "directory of toy projects")
	cmd.Flags().StringVar(&tracker, "tracker", "/demo/tracker", "directory of the tracker's <project>.json issues")
	cmd.Flags().StringVar(&skills, "skills", "/demo/skills", "directory of skills to install")
	cmd.Flags().StringVar(&corpus, "corpus", "/corpus", "directory to export transcripts into")
	cmd.Flags().StringVar(&home, "home", "/home/demo", "the home claude writes into")
	cmd.Flags().StringVar(&src, "src", demo.Root, "where the toy projects are worked on")
	cmd.Flags().StringVar(&only, "only", "", "record only <project> or <project>/<label>")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan and its ceiling, record nothing")
	return cmd
}

func filterScript(s demo.Script, only string) demo.Script {
	if only == "" {
		return s
	}
	project, label, _ := strings.Cut(only, "/")
	if project != s.Project {
		return demo.Script{Project: s.Project}
	}
	if label == "" {
		return s
	}
	kept := demo.Script{Project: s.Project}
	for _, session := range s.Sessions {
		if session.Label == label {
			kept.Sessions = append(kept.Sessions, session)
		}
	}
	return kept
}

func printPlan(out io.Writer, scripts []demo.Script, corpus string) error {
	var ceiling float64
	var prompts int
	for _, s := range scripts {
		for _, session := range s.Sessions {
			state := "to record"
			if _, err := os.Stat(filepath.Join(corpus, s.Project, demo.SessionID(s.Project, session.Label)+".jsonl")); err == nil {
				state = "recorded"
			} else {
				ceiling += session.BudgetUSD * float64(len(session.Prompts))
				prompts += len(session.Prompts)
			}
			fmt.Fprintf(out, "%-12s %-24s %-7s %-10s %d prompt(s) x $%.2f  %s\n",
				s.Project, session.Label, session.Model, session.Visual, len(session.Prompts), session.BudgetUSD, state)
		}
	}
	fmt.Fprintf(out, "\n%d prompts to record; at most $%.2f if every one hits its cap\n", prompts, ceiling)
	return nil
}

func runClaude(ctx context.Context, dir string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func scrubCommand() *cobra.Command {
	corpus := "demo/corpus"
	cmd := &cobra.Command{
		Use:   "scrub",
		Short: "Re-apply the export's scrub to every transcript already in the corpus",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := 0
			err := filepath.WalkDir(corpus, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || filepath.Ext(p) != ".jsonl" {
					return err
				}
				n++
				return demo.Scrub(p)
			})
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "scrubbed %d transcripts in %s\n", n, corpus)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&corpus, "corpus", corpus, "directory of recorded transcripts")
	return cmd
}

func retitleCommand() *cobra.Command {
	corpus, scripts := "demo/corpus", "demo/scripts"
	cmd := &cobra.Command{
		Use:   "retitle",
		Short: "Give recorded sessions the titles their scripts name, as /rename would",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			paths, err := filepath.Glob(filepath.Join(scripts, "*.yaml"))
			if err != nil {
				return err
			}
			for _, p := range paths {
				s, err := demo.LoadScript(p)
				if err != nil {
					return err
				}
				if err := demo.Retitle(corpus, s); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&corpus, "corpus", corpus, "directory of recorded transcripts")
	cmd.Flags().StringVar(&scripts, "scripts", scripts, "directory of session scripts")
	return cmd
}
