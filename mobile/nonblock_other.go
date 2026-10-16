// nonblock_other.go — non-Unix builds: the facade's TUN lifecycle is
// Android-only by purpose; this stub keeps `go build ./...` compilable
// everywhere and StartTun honest (it refuses) if ever called.
//go:build !unix

package mobile

import "errors"

func setNonblock(fd int) error { return errors.New("tun: not supported on this platform") }

// closeFd on non-Unix: nothing to release through this path (StartTun
// refused before owning the fd); the stub keeps the call sites identical.
func closeFd(fd int) {}
