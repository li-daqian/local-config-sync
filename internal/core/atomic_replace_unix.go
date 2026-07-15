//go:build !windows

package core

import "os"

func atomicReplace(source, target string) error { return os.Rename(source, target) }
