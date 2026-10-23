//go:build !linux

package measure

import (
	"errors"
	"time"
)

// threadKernel is zero where a thread's kernel time cannot be read
// apart; nothing is counted there, so Core uses the thread's CPU.
func threadKernel() time.Duration { return 0 }

var errUnsupported = errors.New("instructions are counted only on Linux")

type threadCounters struct{}

func openThread() (*threadCounters, error) { return nil, errUnsupported }

func (c *threadCounters) read() uint64 { return 0 }

func (c *threadCounters) close() {}

// Available says whether this machine can count instructions, and if
// not, why.
func Available() error { return errUnsupported }

// ProcessCounter counts the instructions a process retires.
type ProcessCounter struct{}

// Process counts a process's instructions, which only Linux can.
func Process(int) (*ProcessCounter, error) { return nil, errUnsupported }

func (c *ProcessCounter) Instructions() uint64 { return 0 }

func (c *ProcessCounter) Close() error { return nil }
