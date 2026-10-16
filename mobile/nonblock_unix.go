// nonblock_unix.go — Unix/Android: put a raw fd into non-blocking mode.
package mobile

import "syscall"

// setNonblock marks fd non-blocking. os.NewFile only registers a
// kindNewFile fd with the netpoller when O_NONBLOCK is already set
// (os/file_unix.go: HasNonblockFlag decides); without it the file does
// blocking read(2)s and a Close from another thread can never unblock
// the wire pump — Stop would hang forever. Non-blocking + epoll is the
// standard VpnService TUN pattern (and what production tun2socks runs).
func setNonblock(fd int) error { return syscall.SetNonblock(fd, true) }

// closeFd releases a raw fd refused by StartTun (single-owner rule:
// every path through StartTun consumes the fd).
func closeFd(fd int) { _ = syscall.Close(fd) }
