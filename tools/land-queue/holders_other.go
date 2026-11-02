//go:build !linux

package main

import "syscall"

func dieWithMe() *syscall.SysProcAttr { return nil }

func holders(string) []string { return nil }
