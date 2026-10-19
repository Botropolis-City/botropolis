// Package xvfb starts a private virtual X display. The server picks its
// own display number and reports it through -displayfd, so servers
// started at the same moment never take the same one, which xvfb-run's
// look-then-take did (item 126).
package xvfb

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// Server is one running Xvfb.
type Server struct {
	// Display is the server's display, such as ":12".
	Display string
	// Authority is the file holding the server's cookie.
	Authority string
	// Cookie is the MIT-MAGIC-COOKIE-1 a client must present, in hex.
	Cookie string

	cmd  *exec.Cmd
	once sync.Once
	err  error
}

// Start runs Xvfb with a screen of width by height in a process group of
// its own, refusing any client without the cookie it writes into dir.
func Start(dir string, width, height int) (*Server, error) {
	path, err := exec.LookPath("Xvfb")
	if err != nil {
		return nil, fmt.Errorf("a virtual display needs Xvfb (package xorg-server-xvfb on Arch, xvfb on Debian, xorg-x11-server-Xvfb on Fedora): %w", err)
	}
	cookie := make([]byte, 16)
	if _, err := rand.Read(cookie); err != nil {
		return nil, err
	}
	authority := filepath.Join(dir, "Xauthority")
	if err := os.WriteFile(authority, authorityEntry(cookie), 0o600); err != nil {
		return nil, err
	}
	read, write, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer func() { _ = read.Close() }()
	var stderr bytes.Buffer
	cmd := exec.Command(path, "-displayfd", "3", "-auth", authority, "-nolisten", "tcp",
		"-screen", "0", fmt.Sprintf("%dx%dx24", width, height))
	cmd.ExtraFiles = []*os.File{write}
	cmd.Stderr = &limited{buf: &stderr, max: 4096}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = write.Close()
		return nil, err
	}
	_ = write.Close()
	line, err := bufio.NewReader(read).ReadString('\n')
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("the virtual display did not start: Xvfb reported no display: %s", strings.TrimSpace(stderr.String()))
	}
	return &Server{
		Display:   ":" + strings.TrimSpace(line),
		Authority: authority,
		Cookie:    hex.EncodeToString(cookie),
		cmd:       cmd,
	}, nil
}

// Pid is the server's process, which leads its process group.
func (s *Server) Pid() int { return s.cmd.Process.Pid }

// Env is what a client of this server sets.
func (s *Server) Env() []string {
	return []string{"DISPLAY=" + s.Display, "XAUTHORITY=" + s.Authority}
}

// Close stops the server and waits for it.
func (s *Server) Close() error {
	s.once.Do(func() {
		_ = s.cmd.Process.Kill()
		err := s.cmd.Wait()
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			s.err = err
		}
	})
	return s.err
}

// authorityEntry is one Xauthority record matching any host and any
// display, since the number is not known until the server has taken it.
func authorityEntry(cookie []byte) []byte {
	const familyWild = 0xffff
	var b bytes.Buffer
	field := func(data []byte) {
		_ = binary.Write(&b, binary.BigEndian, uint16(len(data)))
		b.Write(data)
	}
	_ = binary.Write(&b, binary.BigEndian, uint16(familyWild))
	field(nil)
	field(nil)
	field([]byte("MIT-MAGIC-COOKIE-1"))
	field(cookie)
	return b.Bytes()
}

type limited struct {
	mu  sync.Mutex
	buf *bytes.Buffer
	max int
}

func (l *limited) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room := l.max - l.buf.Len(); room > 0 {
		l.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}
