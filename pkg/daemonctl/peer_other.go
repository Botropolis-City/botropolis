//go:build !linux && !darwin

package daemonctl

import "errors"

var errUnsupported = errors.New("this system cannot say which process holds a socket")

func peerPID(int) (int, error) { return 0, errUnsupported }

func cgroup(int) (string, error) { return "", errUnsupported }

func args(int) ([]string, error) { return nil, errUnsupported }
