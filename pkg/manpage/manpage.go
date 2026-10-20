// Package manpage writes botropolis's manual pages, gzipped, from its
// command trees and its config keys, so a command without a page cannot
// ship. Item 167.
package manpage

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/version"
)

const (
	source = "Botropolis"
	manual = "Botropolis Manual"
)

// Write writes a section 1 page for root and each command under it that
// help lists, named by command path, such as botropolis-status.1.gz.
func Write(dir string, date time.Time, root *cobra.Command) error {
	tmp, err := os.MkdirTemp("", "botropolis-man")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	quiet(root)
	header := &doc.GenManHeader{Section: "1", Date: &date, Source: source + " " + version.Version, Manual: manual}
	if err := doc.GenManTreeFromOpts(root, doc.GenManTreeOptions{Header: header, Path: tmp, CommandSeparator: "-"}); err != nil {
		return err
	}
	pages, err := filepath.Glob(filepath.Join(tmp, "*.1"))
	if err != nil {
		return err
	}
	for _, p := range pages {
		text, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := writeGzip(filepath.Join(dir, filepath.Base(p)+".gz"), text); err != nil {
			return err
		}
	}
	return nil
}

// quiet drops cobra's generator credit, and escapes the angle brackets of
// arguments such as <id>, which the generator's Markdown would otherwise
// take for HTML and drop. It changes the tree it is given.
func quiet(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	escape := strings.NewReplacer("<", `\<`, ">", `\>`)
	cmd.Use = escape.Replace(cmd.Use)
	cmd.Long = escape.Replace(cmd.Long)
	for _, c := range cmd.Commands() {
		quiet(c)
	}
}

// WriteConfig writes botropolis-config(5), the config file's keys.
func WriteConfig(dir string, date time.Time, docs []config.KeyDoc) error {
	var b strings.Builder
	fmt.Fprintf(&b, ".nh\n.TH \"BOTROPOLIS-CONFIG\" \"5\" %q %q %q\n", date.Format("Jan 2006"), source+" "+version.Version, manual)
	b.WriteString(".SH NAME\nbotropolis-config \\- the botropolis configuration file\n")
	b.WriteString(".SH SYNOPSIS\n\\fI$XDG_CONFIG_HOME/botropolis/config.toml\\fP, by default \\fI~/.config/botropolis/config.toml\\fP\n")
	b.WriteString(".SH DESCRIPTION\n" + roff("Botropolis reads its settings from config.toml, config.yaml or config.json in that directory, or from the file --config names. The city's settings panel, s, writes the same file.") + "\n")
	b.WriteString(".PP\n" + roff("Any key can be set for one run with --set key=value, which can be repeated, or with its environment variable. --set wins over the environment, and the environment over the file.") + "\n")
	b.WriteString(".SH KEYS\n")
	for _, d := range docs {
		def := "none"
		if d.Default != "" {
			def = d.Default
		}
		fmt.Fprintf(&b, ".TP\n\\fB%s\\fP\n%s\n.br\n%s\n", roff(d.Key), roff(d.Help), roff("Default: "+def+". Environment: "+d.Env+"."))
	}
	b.WriteString(".SH SEE ALSO\n\\fBbotropolis(1)\\fP\n")
	return writeGzip(filepath.Join(dir, "botropolis-config.5.gz"), []byte(b.String()))
}

func roff(s string) string {
	s = strings.ReplaceAll(s, `\`, `\e`)
	return strings.ReplaceAll(s, "-", `\-`)
}

// writeGzip compresses with no name and no time in the header, so the
// same page is the same bytes on every build.
func writeGzip(path string, text []byte) error {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := w.Write(text); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
