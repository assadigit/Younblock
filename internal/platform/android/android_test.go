package android

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/netx"
)

// fakeProtector counts protect calls and can be told to fail.
type fakeProtector struct {
	mu      chan struct{}
	calls   int
	failAll bool
}

func (f *fakeProtector) protect(fd int32) bool {
	f.mu <- struct{}{}
	defer func() { <-f.mu }()
	f.calls++
	if f.failAll {
		return false
	}
	return true
}

func (f *fakeProtector) count() int {
	f.mu <- struct{}{}
	defer func() { <-f.mu }()
	return f.calls
}

func newFakeProtector() *fakeProtector {
	return &fakeProtector{mu: make(chan struct{}, 1)}
}

func TestNameAndCapabilities(t *testing.T) {
	p := New(Options{})
	if got := p.Name(); got != "android" {
		t.Fatalf("Name() = %q, want %q", got, "android")
	}
	caps := p.Capabilities()
	if !caps.PerAppRouting {
		t.Error("Capabilities().PerAppRouting = false, want true (the VpnService owns per-app routing; doc 04 §2.4)")
	}
	// M8-am (ADR-0051, D105): the connect-time hand-off ships on Android
	// — the capability gates the UI's auto-open row and the setting's
	// readers. Both-ways proof: with the field stashed, this FAILS
	// (false) and the MenuSheet row never renders.
	if !caps.AutoOpenTarget {
		t.Error("Capabilities().AutoOpenTarget = false, want true (the connect-time YouTube hand-off; M8-am)")
	}
	if caps.SystemProxy || caps.Tray || caps.StartWithOS || caps.BrowserSession {
		t.Errorf("Capabilities() = %+v: desktop-only capabilities must all be false on Android", caps)
	}
}

func TestNoOpPlatformSurface(t *testing.T) {
	p := New(Options{})
	s := config.Settings{Mode: config.ModeYouTube}
	if err := p.Attach(4525, s); err != nil {
		t.Errorf("Attach() = %v, want nil (routing is the VpnService's)", err)
	}
	rewrote, err := p.EnsureAttached(4525, s)
	if err != nil || rewrote {
		t.Errorf("EnsureAttached() = (%v, %v), want (false, nil)", rewrote, err)
	}
	if err := p.Detach(); err != nil {
		t.Errorf("Detach() = %v, want nil", err)
	}
	if err := p.SetAutostart(true); err != nil {
		t.Errorf("SetAutostart() = %v, want nil (AND-028: nothing auto-starts)", err)
	}
	if err := p.RestoreProxy(); err != nil {
		t.Errorf("RestoreProxy() = %v, want nil", err)
	}
}

func TestConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "younblock")
	p := New(Options{ConfigDir: dir})
	if got := p.ConfigDir(); got != dir {
		t.Fatalf("ConfigDir() = %q, want %q", got, dir)
	}
}

func TestNotifyCallbackForwardedAndDropped(t *testing.T) {
	type notice struct {
		kind engine.NoticeKind
		text string
	}
	got := make(chan notice, 1)
	p := New(Options{Notify: func(kind engine.NoticeKind, text string) {
		got <- notice{kind, text}
	}})
	p.Notify(engine.NoticeFailure, "secure dns is unreachable")
	select {
	case n := <-got:
		if n.kind != engine.NoticeFailure || n.text != "secure dns is unreachable" {
			t.Fatalf("forwarded notice = %+v", n)
		}
	case <-time.After(time.Second):
		t.Fatal("Notify callback was not forwarded")
	}
	// Without a callback, Notify must be a silent no-op.
	New(Options{}).Notify(engine.NoticeRecovery, "recovered")
}

func TestDialControlNilWithoutProtector(t *testing.T) {
	p := New(Options{})
	if got := p.DialControl(); got != nil {
		t.Fatalf("DialControl() = %v, want nil when no protector is configured", got)
	}
}

// TestProtectCountsEveryDial is the M3 host half of ADR-0011's mandate
// ("test that every dial calls it"): with the adapter's DialControl
// installed on the engine's dialer shape (netx.Dialer), every address
// attempt — successful or failed — invokes protect exactly once before
// connecting, and a refused protect aborts the dial.
func TestProtectCountsEveryDial(t *testing.T) {
	fp := newFakeProtector()
	p := New(Options{Protect: fp.protect})
	control := p.DialControl()
	if control == nil {
		t.Fatal("DialControl() = nil, want the protect hook")
	}
	d := netx.New(500*time.Millisecond, 2*time.Second, control)

	// 1. A successful dial is protected exactly once.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	tcpAddr := ln.Addr().(*net.TCPAddr)
	conn, err := d.DialHost(t.Context(), "tcp", "test.local", tcpAddr.Port, []net.IP{tcpAddr.IP})
	if err != nil {
		t.Fatalf("dial to the local listener failed: %v", err)
	}
	conn.Close()
	if got := fp.count(); got != 1 {
		t.Fatalf("after one successful dial, protect called %d times, want 1", got)
	}

	// 2. A failed attempt still protects the socket (the hook runs before
	// connect; the fd exists even when the connect is refused).
	closed := ln
	closed.Close()
	port := tcpAddr.Port
	_, err = d.DialHost(t.Context(), "tcp", "test.local", port, []net.IP{tcpAddr.IP})
	if err == nil {
		t.Fatal("dial to a closed port unexpectedly succeeded")
	}
	if got := fp.count(); got != 2 {
		t.Fatalf("after the failed dial, protect called %d times, want 2 (every dial, failed or not, must be protected)", got)
	}

	// 3. A multi-address run protects each per-IP attempt. The two
	// addresses must be genuinely different: the whole 127/8 block is
	// loopback on Linux, so 127.0.0.2 gives a refused attempt (nothing
	// listens there) while 127.0.0.1 carries the live listener. With the
	// same IP twice, the first "attempt" would simply connect.
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln2.Close()
	go func() {
		for {
			c, err := ln2.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	addr2 := ln2.Addr().(*net.TCPAddr)
	deadIP := net.ParseIP("127.0.0.2")
	if deadIP == nil {
		t.Fatal("127.0.0.2 did not parse")
	}
	before := fp.count()
	conn, err = d.DialHost(t.Context(), "tcp", "test.local", addr2.Port, []net.IP{deadIP, addr2.IP})
	if err != nil {
		t.Fatalf("multi-address dial failed: %v", err)
	}
	conn.Close()
	// The first attempt hit the dead address (refused, 1 call), the second
	// connected (1 call).
	if got, want := fp.count()-before, 2; got != want {
		t.Fatalf("multi-address run: protect called %d times, want %d", got, want)
	}
}

// TestProtectRefusalAbortsDial: when the shell refuses to protect, the
// dial must fail (a failed protect means the socket would loop back into
// the TUN — fail safe, never connect unprotected).
func TestProtectRefusalAbortsDial(t *testing.T) {
	fp := newFakeProtector()
	fp.failAll = true
	p := New(Options{Protect: fp.protect})
	d := netx.New(500*time.Millisecond, 2*time.Second, p.DialControl())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	accepted := make(chan net.Conn, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted <- c
		}
	}()
	tcpAddr := ln.Addr().(*net.TCPAddr)
	_, err = d.DialHost(t.Context(), "tcp", "test.local", tcpAddr.Port, []net.IP{tcpAddr.IP})
	if err == nil {
		t.Fatal("dial succeeded despite protect() refusing — must abort")
	}
	if !strings.Contains(err.Error(), "protect failed") {
		t.Fatalf("dial error = %v, want it to mention the protect failure", err)
	}
	// Nothing may have connected: no accepted connection pending.
	select {
	case c := <-accepted:
		c.Close()
		t.Fatal("a connection reached the listener despite protect refusal")
	default:
	}
}
