// Command core is the headless Younblock engine (file 01, section 3):
// younblock-core.exe with --port, --preset, --selftest, --restore-proxy
// and --version.
package main

import (
        "context"
        "flag"
        "fmt"
        "os"
        "os/signal"
        "syscall"
        "time"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/engine"
        "github.com/assadigit/Younblock/internal/health"
        "github.com/assadigit/Younblock/internal/platform"
)

func main() {
        port := flag.Int("port", 0, "listen port (0 = use the saved setting)")
        preset := flag.String("preset", "", "fragmentation preset: light, balanced, strong or legacy")
        selftest := flag.Bool("selftest", false, "run the four-step connection test, print it and exit")
        restore := flag.Bool("restore-proxy", false, "restore the Windows internet settings and exit")
        version := flag.Bool("version", false, "print the version and exit")
        flag.Parse()

        if *version {
                fmt.Printf("%s core %s\n", config.AppName, config.AppVersion)
                return
        }

        plat := platform.New()

        if *restore {
                if err := plat.RestoreProxy(); err != nil {
                        fmt.Println("Restore failed:", err)
                        os.Exit(1)
                }
                fmt.Println("Internet settings restored.")
                return
        }

        opts := engine.Options{
                Platform:       plat,
                PortOverride:   *port,
                MethodOverride: config.Method(*preset),
        }
        eng, err := engine.New(opts)
        if err != nil {
                fmt.Println("Startup failed:", err)
                os.Exit(1)
        }
        defer eng.Shutdown()

        if *selftest {
                runSelfTestAndExit(eng)
        }

        eng.Start()

        sig := make(chan os.Signal, 1)
        signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
        fmt.Printf("%s core %s running. Press Ctrl+C to stop.\n", config.AppName, config.AppVersion)
        <-sig
        fmt.Println("Shutting down...")
        eng.Stop()
}

// runSelfTestAndExit starts the engine, runs the four checks, prints the
// result and exits with a non-zero code when any step failed.
func runSelfTestAndExit(eng *engine.Engine) {
        eng.Start()
        // Wait for the engine to reach a terminal phase (on or error).
        deadline := time.Now().Add(10 * time.Second)
        for time.Now().Before(deadline) {
                if st := eng.GetState(); st.Phase == engine.PhaseOn || st.Phase == engine.PhaseError {
                        break
                }
                time.Sleep(50 * time.Millisecond)
        }
        st := eng.GetState()
        fmt.Printf("State: %s (port %d)\n", st.Phase, st.Port)
        srv := eng.RunningServer()
        if srv == nil {
                fmt.Println("Self-test skipped: the proxy is not running.")
                eng.Stop()
                os.Exit(1)
        }
        steps := health.RunSelfTest(context.Background(), health.Config{
                ProxyAddr: srv.Addr(),
                Resolver:  eng.Resolver(),
                Timeout:   10 * time.Second,
        }, func(s health.Step) {
                mark := "FAIL"
                if s.OK {
                        mark = "OK  "
                }
                fmt.Printf("  [%s] %-8s %4d ms  %s\n", mark, s.ID, s.MS, s.Detail)
        })
        eng.Stop()
        if health.AllOK(steps) {
                fmt.Println("Self-test passed.")
                os.Exit(0)
        }
        fmt.Println("Self-test FAILED.")
        os.Exit(1)
}
