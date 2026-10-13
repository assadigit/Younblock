package engine_test

import (
        "bufio"
        "crypto/ecdsa"
        "crypto/elliptic"
        "crypto/rand"
        "crypto/tls"
        "crypto/x509"
        "crypto/x509/pkix"
        "encoding/base64"
        "encoding/json"
        "fmt"
        "math/big"
        "net"
        "net/http"
        "strings"
        "sync"
        "testing"
        "time"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/engine"
        "github.com/assadigit/Younblock/internal/health"
        "github.com/assadigit/Younblock/internal/platform/stub"

        "golang.org/x/net/dns/dnsmessage"
)

// waitForState polls until the condition holds.
func waitForState(t *testing.T, eng *engine.Engine, timeout time.Duration, want func(engine.State) bool) engine.State {
        t.Helper()
        deadline := time.Now().Add(timeout)
        for time.Now().Before(deadline) {
                st := eng.GetState()
                if want(st) {
                        return st
                }
                time.Sleep(10 * time.Millisecond)
        }
        t.Fatalf("state never satisfied the condition; now %+v", eng.GetState())
        return engine.State{}
}

func phaseOn(s engine.State) bool       { return s.Phase == engine.PhaseOn }
func phaseDegraded(s engine.State) bool { return s.Phase == engine.PhaseDegraded }
func phaseOff(s engine.State) bool      { return s.Phase == engine.PhaseOff }

// ---- Phase 2 acceptance: fake networks that only accept one preset ----

func TestStartHealSwitchesMethodAndAnnouncesNotice(t *testing.T) {
        eng, _, sink, net := newTestEngine(t)
        net.acceptsOnly(config.MethodStrong) // balanced and everything else fail

        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        st := waitForState(t, eng, 5*time.Second, phaseOn)
        if st.Method != config.MethodStrong {
                t.Errorf("method = %s, want strong (the only method the fake network accepts)", st.Method)
        }
        if st.Notice == nil || st.Notice.Code != "METHOD_SWITCHED" || st.Notice.Arg != "strong" {
                t.Errorf("notice = %+v, want METHOD_SWITCHED(strong)", st.Notice)
        }
        // The sweep must have announced each attempt through detail changes.
        var sawTrying bool
        for _, payload := range sink.states() {
                var s engine.State
                if json.Unmarshal([]byte(payload), &s) == nil && strings.HasPrefix(s.Detail, "trying:") {
                        sawTrying = true
                }
        }
        if !sawTrying {
                t.Error("no trying:<preset> state was announced during the sweep")
        }
        // The working method is persisted (file 01 section 10).
        saved, _, err := config.Load(eng.ConfigDir())
        if err != nil || saved.Method != config.MethodStrong {
                t.Errorf("persisted method = %s (err %v), want strong", saved.Method, err)
        }
}

func TestStartNoMethodWorksGoesDegradedListenerStaysUp(t *testing.T) {
        eng, plat, _, net := newTestEngine(t)
        net.acceptAll(false)

        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        st := waitForState(t, eng, 5*time.Second, phaseDegraded)
        if st.Error == nil || st.Error.Code != "SELFTEST_FAILED" {
                t.Errorf("state error = %+v, want SELFTEST_FAILED", st.Error)
        }
        // The listener keeps running (file 01, section 10).
        if eng.RunningServer() == nil {
                t.Fatal("listener should keep running in degraded state")
        }
        // One failure notification ("Couldn't fix it. Open Younblock.").
        found := false
        for _, n := range plat.Notices() {
                if n.Kind == engine.NoticeFailure && strings.Contains(n.Text, "Couldn't fix it") {
                        found = true
                }
        }
        if !found {
                t.Errorf("notifications = %+v, want the failure text", plat.Notices())
        }
}

func TestStartTotalTimeoutCap(t *testing.T) {
        dir := t.TempDir()
        plat := stub.NewWithDir(dir)
        net := &fakeNetwork{delay: 400 * time.Millisecond}
        net.acceptAll(false)
        eng, err := engine.New(engine.Options{
                Platform:     plat,
                LogDir:       dir + "/logs",
                Checker:      net,
                StartTimeout: 1200 * time.Millisecond, // far below the real 35 s
        })
        if err != nil {
                t.Fatal(err)
        }
        t.Cleanup(eng.Shutdown)
        start := time.Now()
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        st := waitForState(t, eng, 5*time.Second, func(s engine.State) bool {
                return s.Phase == engine.PhaseDegraded || s.Phase == engine.PhaseError
        })
        elapsed := time.Since(start)
        if elapsed > 2500*time.Millisecond {
                t.Errorf("final answer took %v, want at most ~2.5x the 1.2 s cap", elapsed)
        }
        if st.Error == nil || st.Error.Code != "SELFTEST_FAILED" {
                t.Errorf("state = %+v, want SELFTEST_FAILED after the cap", st)
        }
}

func TestHealSweepCooldownBlocksSecondAutomaticSweep(t *testing.T) {
        dir := t.TempDir()
        plat := stub.NewWithDir(dir)
        net := &fakeNetwork{}
        net.acceptAll(false)
        eng, err := engine.New(engine.Options{
                Platform:     plat,
                LogDir:       dir + "/logs",
                Checker:      net,
                HealCooldown: 400 * time.Millisecond,
        })
        if err != nil {
                t.Fatal(err)
        }
        t.Cleanup(eng.Shutdown)

        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 5*time.Second, phaseDegraded)
        firstHeals := net.healCallsCount()

        // A second Start within the cooldown must NOT sweep again.
        if err := eng.Stop(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 2*time.Second, phaseOff)
        time.Sleep(50 * time.Millisecond)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 5*time.Second, phaseDegraded)
        if got := net.healCallsCount(); got != firstHeals {
                t.Errorf("heal calls during cooldown = %d, want still %d", got, firstHeals)
        }

        // After the cooldown passes, the next Start may sweep again.
        time.Sleep(450 * time.Millisecond)
        if err := eng.Stop(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 2*time.Second, phaseOff)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 5*time.Second, phaseDegraded)
        if got := net.healCallsCount(); got <= firstHeals {
                t.Errorf("heal calls after cooldown = %d, want more than %d", got, firstHeals)
        }
}

func TestWatchdogHealsAndRecovers(t *testing.T) {
        dir := t.TempDir()
        plat := stub.NewWithDir(dir)
        net := &fakeNetwork{}
        net.acceptAll(true)
        eng, err := engine.New(engine.Options{
                Platform:          plat,
                LogDir:            dir + "/logs",
                Checker:           net,
                WatchdogInterval:  40 * time.Millisecond,
                WatchdogFailLimit: 2,
                HealCooldown:      50 * time.Millisecond,
        })
        if err != nil {
                t.Fatal(err)
        }
        t.Cleanup(eng.Shutdown)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 5*time.Second, phaseOn)

        // The network breaks: watchdog failures trigger a sweep, which fails,
        // so the engine degrades with notifications (R14).
        net.acceptAll(false)
        st := waitForState(t, eng, 5*time.Second, phaseDegraded)
        if st.Error == nil || st.Error.Code != "SELFTEST_FAILED" {
                t.Errorf("degraded state = %+v", st)
        }
        sawTrying, sawFailed := false, false
        for _, n := range plat.Notices() {
                if strings.Contains(n.Text, "Trying to fix it") {
                        sawTrying = true
                }
                if strings.Contains(n.Text, "Couldn't fix it") {
                        sawFailed = true
                }
        }
        if !sawTrying || !sawFailed {
                t.Errorf("notifications = %+v, want both the trying and failed texts", plat.Notices())
        }

        // The network recovers: one later watchdog success returns to on with a
        // RECOVERED notice and a "Fixed" notification.
        net.acceptAll(true)
        st = waitForState(t, eng, 5*time.Second, phaseOn)
        if st.Notice == nil || st.Notice.Code != "RECOVERED" {
                t.Errorf("notice after recovery = %+v, want RECOVERED", st.Notice)
        }
        sawFixed := false
        for _, n := range plat.Notices() {
                if n.Kind == engine.NoticeRecovery && n.Text == "Fixed" {
                        sawFixed = true
                }
        }
        if !sawFixed {
                t.Errorf("missing the Fixed notification: %+v", plat.Notices())
        }
}

func TestWatchdogSweepSwitchesMethodAndAnnounces(t *testing.T) {
        dir := t.TempDir()
        plat := stub.NewWithDir(dir)
        net := &fakeNetwork{}
        net.acceptAll(true)
        eng, err := engine.New(engine.Options{
                Platform:          plat,
                LogDir:            dir + "/logs",
                Checker:           net,
                WatchdogInterval:  40 * time.Millisecond,
                WatchdogFailLimit: 2,
                HealCooldown:      50 * time.Millisecond,
        })
        if err != nil {
                t.Fatal(err)
        }
        t.Cleanup(eng.Shutdown)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 5*time.Second, phaseOn)

        // The network now only accepts light: the watchdog sweep must switch to
        // it and announce METHOD_SWITCHED (nothing changes silently).
        net.acceptsOnly(config.MethodLight)
        st := waitForState(t, eng, 5*time.Second, func(s engine.State) bool {
                return s.Phase == engine.PhaseOn && s.Method == config.MethodLight
        })
        if st.Notice == nil || st.Notice.Code != "METHOD_SWITCHED" || st.Notice.Arg != "light" {
                t.Errorf("notice = %+v, want METHOD_SWITCHED(light)", st.Notice)
        }
        saved, _, err := config.Load(eng.ConfigDir())
        if err != nil || saved.Method != config.MethodLight {
                t.Errorf("persisted method = %s (err %v), want light", saved.Method, err)
        }
}

func TestAutoFixFromDegradedReturnsWorkingMethod(t *testing.T) {
        eng, _, _, net := newTestEngine(t)
        net.acceptAll(false)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 5*time.Second, phaseDegraded)

        // The network now accepts only light: AutoFix must find it.
        net.acceptsOnly(config.MethodLight)
        out := eng.AutoFixJSON()
        var res struct {
                Method string `json:"method"`
                Error  *struct {
                        Code string `json:"code"`
                } `json:"error"`
        }
        if err := json.Unmarshal([]byte(out), &res); err != nil {
                t.Fatalf("AutoFix reply: %s", out)
        }
        if res.Error != nil || res.Method != "light" {
                t.Fatalf("AutoFix reply = %+v, want method light", res)
        }
        st := waitForState(t, eng, 5*time.Second, phaseOn)
        if st.Method != config.MethodLight {
                t.Errorf("method = %s, want light", st.Method)
        }
        if st.Notice == nil || st.Notice.Code != "METHOD_SWITCHED" {
                t.Errorf("notice = %+v, want METHOD_SWITCHED", st.Notice)
        }
}

func TestAutoFixFromOffStarts(t *testing.T) {
        eng, _, _, net := newTestEngine(t)
        net.acceptAll(true)
        out := eng.AutoFixJSON()
        if !strings.Contains(out, `"method"`) {
                t.Fatalf("AutoFix from off = %s", out)
        }
        waitForState(t, eng, 5*time.Second, phaseOn)
}

func TestStopDuringConnectingCancelsCleanly(t *testing.T) {
        dir := t.TempDir()
        plat := stub.NewWithDir(dir)
        net := &fakeNetwork{delay: 250 * time.Millisecond}
        net.acceptAll(false)
        eng, err := engine.New(engine.Options{
                Platform: plat,
                LogDir:   dir + "/logs",
                Checker:  net,
        })
        if err != nil {
                t.Fatal(err)
        }
        t.Cleanup(eng.Shutdown)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        // Press the button again while connecting: cancels cleanly (file 01 §10.5).
        time.Sleep(80 * time.Millisecond)
        if err := eng.Toggle(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 3*time.Second, phaseOff)
        // And it must not later flip to degraded from the aborted sweep.
        time.Sleep(400 * time.Millisecond)
        if st := eng.GetState(); st.Phase != engine.PhaseOff {
                t.Errorf("phase drifted to %s after cancel", st.Phase)
        }
}

func TestNoticeExpiresAfterTenSeconds(t *testing.T) {
        dir := t.TempDir()
        plat := stub.NewWithDir(dir)
        mu := sync.Mutex{}
        now := time.Now()
        clock := func() time.Time {
                mu.Lock()
                defer mu.Unlock()
                return now
        }
        net := &fakeNetwork{}
        net.acceptsOnly(config.MethodStrong)
        eng, err := engine.New(engine.Options{
                Platform: plat,
                LogDir:   dir + "/logs",
                Checker:  net,
                Now:      clock,
        })
        if err != nil {
                t.Fatal(err)
        }
        t.Cleanup(eng.Shutdown)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 5*time.Second, phaseOn)
        if st := eng.GetState(); st.Notice == nil {
                t.Fatal("notice should be visible right after the method switch")
        }
        // Advance the clock past the 10 s notice window.
        mu.Lock()
        now = now.Add(11 * time.Second)
        mu.Unlock()
        if st := eng.GetState(); st.Notice != nil {
                t.Errorf("notice still visible after 11 s: %+v", st.Notice)
        }
}

// ---- R19: network change resets DNS and re-checks ----

func TestNetworkChangeResetsDNSAndRechecks(t *testing.T) {
        dir := t.TempDir()
        plat := stub.NewWithDir(dir)
        fake := &fakeNetwork{}
        fake.acceptAll(true)
        mu := sync.Mutex{}
        addrs := []net.Addr{&net.IPNet{IP: net.ParseIP("192.168.1.2"), Mask: net.CIDRMask(24, 32)}}
        lister := func() ([]net.Addr, error) {
                mu.Lock()
                defer mu.Unlock()
                return addrs, nil
        }
        eng, err := engine.New(engine.Options{
                Platform:            plat,
                LogDir:              dir + "/logs",
                Checker:             fake,
                NetworkPollInterval: 40 * time.Millisecond,
                InterfaceLister:     lister,
                WatchdogInterval:    time.Hour, // no periodic ticks: only the immediate check counts
        })
        if err != nil {
                t.Fatal(err)
        }
        t.Cleanup(eng.Shutdown)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 5*time.Second, phaseOn)
        resetsBefore := eng.Resolver().ResetCount()
        callsBefore := fake.callCount()

        // The interface set changes (Wi-Fi -> Ethernet).
        mu.Lock()
        addrs = []net.Addr{&net.IPNet{IP: net.ParseIP("10.0.0.5"), Mask: net.CIDRMask(24, 32)}}
        mu.Unlock()

        deadline := time.Now().Add(2 * time.Second)
        for time.Now().Before(deadline) {
                if eng.Resolver().ResetCount() > resetsBefore && fake.callCount() > callsBefore {
                        return // both effects observed
                }
                time.Sleep(10 * time.Millisecond)
        }
        t.Errorf("network change effects missing: resets %d->%d, checks %d->%d",
                resetsBefore, eng.Resolver().ResetCount(), callsBefore, fake.callCount())
}

// ---- RunConnectionTest with progress events (hermetic local network) ----

// localNet is a self-contained fake internet: DoH + web + video servers.
type localNet struct {
        pool       *x509.CertPool
        dohURL     string
        primaryURL string
        fallback   string
        videoHost  string
}

func newLocalNet(t *testing.T) *localNet {
        t.Helper()
        // CA and one leaf covering everything.
        caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
        if err != nil {
                t.Fatal(err)
        }
        caTmpl := &x509.Certificate{
                SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "engine test CA"},
                NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
                IsCA: true, KeyUsage: x509.KeyUsageCertSign, BasicConstraintsValid: true,
        }
        caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, caKey.Public(), caKey)
        if err != nil {
                t.Fatal(err)
        }
        caCert, _ := x509.ParseCertificate(caDER)
        pool := x509.NewCertPool()
        pool.AddCert(caCert)

        leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
        if err != nil {
                t.Fatal(err)
        }
        leafTmpl := &x509.Certificate{
                SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "engine test leaf"},
                NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
                DNSNames:    []string{"yt.test", "video.test"},
                IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
                KeyUsage:    x509.KeyUsageDigitalSignature,
                ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
        }
        leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, leafKey.Public(), caKey)
        if err != nil {
                t.Fatal(err)
        }
        leaf := tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}

        // Fake DoH.
        dohLn, err := net.Listen("tcp", "127.0.0.1:0")
        if err != nil {
                t.Fatal(err)
        }
        dohTLS := tls.NewListener(dohLn, &tls.Config{Certificates: []tls.Certificate{leaf}})
        dohMux := http.NewServeMux()
        dohMux.HandleFunc("/dns-query", func(w http.ResponseWriter, r *http.Request) {
                wire, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(r.URL.Query().Get("dns"))
                if err != nil {
                        w.WriteHeader(http.StatusBadRequest)
                        return
                }
                var p dnsmessage.Parser
                hdr, err := p.Start(wire)
                if err != nil {
                        w.WriteHeader(http.StatusBadRequest)
                        return
                }
                question, err := p.Question()
                if err != nil {
                        w.WriteHeader(http.StatusBadRequest)
                        return
                }
                b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: hdr.ID, Response: true, RCode: dnsmessage.RCodeSuccess})
                b.StartQuestions()
                b.Question(question)
                b.StartAnswers()
                v4 := net.ParseIP("127.0.0.1").To4()
                b.AResource(dnsmessage.ResourceHeader{
                        Name: question.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 300,
                }, dnsmessage.AResource{A: [4]byte{v4[0], v4[1], v4[2], v4[3]}})
                msg, _ := b.Finish()
                w.Header().Set("content-type", "application/dns-message")
                w.Write(msg)
        })
        dohSrv := &http.Server{Handler: dohMux}
        go dohSrv.Serve(dohTLS)
        t.Cleanup(func() { dohSrv.Close(); dohLn.Close() })

        // Web + video server (one TLS listener serves both roles).
        webLn, err := net.Listen("tcp", "127.0.0.1:0")
        if err != nil {
                t.Fatal(err)
        }
        webPort := webLn.Addr().(*net.TCPAddr).Port
        go func() {
                for {
                        c, err := webLn.Accept()
                        if err != nil {
                                return
                        }
                        go func(c net.Conn) {
                                defer c.Close()
                                tc := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{leaf}})
                                tc.SetDeadline(time.Now().Add(10 * time.Second))
                                br := bufio.NewReader(tc)
                                req, err := http.ReadRequest(br)
                                if err != nil {
                                        return // the video step only handshakes; that is fine
                                }
                                status := http.StatusOK
                                if req.URL.Path == "/generate_204" {
                                        status = http.StatusNoContent
                                }
                                fmt.Fprintf(tc, "HTTP/1.1 %d X\r\nContent-Length: 0\r\n\r\n", status)
                                tc.CloseWrite()
                        }(c)
                }
        }()
        t.Cleanup(func() { webLn.Close() })

        return &localNet{
                pool:       pool,
                dohURL:     "https://" + dohTLS.Addr().String(),
                primaryURL: fmt.Sprintf("https://yt.test:%d/generate_204", webPort),
                fallback:   fmt.Sprintf("https://yt.test:%d/robots.txt", webPort),
                videoHost:  fmt.Sprintf("video.test:%d", webPort),
        }
}

func TestRunConnectionTestEmitsProgress(t *testing.T) {
        env := newLocalNet(t)
        dir := t.TempDir()
        plat := stub.NewWithDir(dir)
        // Settings point the resolver at the local fake DoH.
        settings := config.DefaultSettings()
        settings.Advanced.DoHServers = []config.DoHServer{{
                Name: "FakeDoH", URL: env.dohURL + "/dns-query?dns={dns}", Bootstrap: []string{"127.0.0.1"},
        }}
        if err := config.Save(dir, settings); err != nil {
                t.Fatal(err)
        }
        fake := &fakeNetwork{}
        fake.acceptAll(true)
        eng, err := engine.New(engine.Options{
                Platform:            plat,
                LogDir:              dir + "/logs",
                Checker:             fake,
                AllowPrivateTargets: true, // the fake network resolves to 127.0.0.1
                ResolverTLS:         &tls.Config{RootCAs: env.pool},
                HealthOverrides: func(hc *health.Config) {
                        hc.PrimaryURL = env.primaryURL
                        hc.FallbackURL = env.fallback
                        hc.VideoHost = env.videoHost
                        hc.WebTLS = &tls.Config{RootCAs: env.pool}
                        hc.VideoTLS = &tls.Config{RootCAs: env.pool}
                },
        })
        if err != nil {
                t.Fatal(err)
        }
        t.Cleanup(eng.Shutdown)
        sink := &recordingSink{}
        eng.SetEventSink(sink)
        if err := eng.Start(); err != nil {
                t.Fatal(err)
        }
        waitForState(t, eng, 5*time.Second, phaseOn)

        out := eng.RunConnectionTestJSON()
        var steps []struct {
                ID string `json:"id"`
                OK bool   `json:"ok"`
        }
        if err := json.Unmarshal([]byte(out), &steps); err != nil {
                t.Fatalf("RunConnectionTest reply: %s", out)
        }
        if len(steps) != 4 {
                t.Fatalf("steps = %d, want 4", len(steps))
        }
        wantIDs := []string{"listener", "dns", "youtube", "video"}
        for i, s := range steps {
                if s.ID != wantIDs[i] {
                        t.Errorf("step %d = %q, want %q", i, s.ID, wantIDs[i])
                }
                if !s.OK {
                        t.Errorf("step %s unexpectedly failed on the local fake network", s.ID)
                }
        }
        // One progress event per step.
        progress := 0
        for _, e := range sink.all() {
                if e.name == "test:progress" {
                        progress++
                }
        }
        if progress != 4 {
                t.Errorf("test:progress events = %d, want 4", progress)
        }
        // The diagnostic report now contains the self-test summary.
        if rep := eng.GetDiagnosticReport(); !strings.Contains(rep, "self-test:") {
                t.Errorf("diagnostic report missing the self-test summary")
        }
}
