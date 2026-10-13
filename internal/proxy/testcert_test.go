package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// testCA makes a throwaway CA for local TLS servers in tests.
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pool *x509.CertPool
}

func newTestCA(t interface {
	Fatalf(format string, args ...any)
	Helper()
}) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Younblock Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatalf("ca cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse ca: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &testCA{cert: cert, key: key, pool: pool}
}

// serverCert issues a leaf certificate valid for the given DNS names and IPs.
func (ca *testCA) serverCert(t interface {
	Fatalf(format string, args ...any)
	Helper()
}, dnsNames []string, ips []net.IP) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "Younblock Test Leaf"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     dnsNames,
		IPAddresses:  ips,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, key.Public(), ca.key)
	if err != nil {
		t.Fatalf("leaf cert: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// recordingListener wraps every accepted connection in a recordingConn,
// which counts the TCP reads that carry data and how many of them happened
// before the server's first write (i.e. while consuming the ClientHello).
type recordingListener struct {
	net.Listener
	mu    sync.Mutex
	conns []*recordingConn
}

func (l *recordingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	rc := &recordingConn{Conn: c}
	l.mu.Lock()
	l.conns = append(l.conns, rc)
	l.mu.Unlock()
	return rc, nil
}

// latest returns the most recently accepted recording connection.
func (l *recordingListener) latest() *recordingConn {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.conns) == 0 {
		return nil
	}
	return l.conns[len(l.conns)-1]
}

type recordingConn struct {
	net.Conn
	reads     atomic.Int64
	wrote     atomic.Bool
	preWrites atomic.Int64 // reads that returned data before the first write
}

func (c *recordingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		c.reads.Add(1)
		if !c.wrote.Load() {
			c.preWrites.Add(1)
		}
	}
	return n, err
}

func (c *recordingConn) Write(b []byte) (int, error) {
	c.wrote.Store(true)
	return c.Conn.Write(b)
}

func (c *recordingConn) String() string {
	return fmt.Sprintf("recordingConn(reads=%d preWrites=%d)", c.reads.Load(), c.preWrites.Load())
}

func (c *recordingConn) stats() (reads int64, preWrites int64) {
	return c.reads.Load(), c.preWrites.Load()
}
