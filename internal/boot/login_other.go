//go:build !windows

package boot

import "errors"

func Register(exe, data string, enabled bool) error { return nil }
func ID() (string, error)                           { return "", errors.New("startup tasks require Windows") }
