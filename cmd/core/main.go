// Command core is the headless Younblock engine (file 01, section 3):
// younblock-core.exe with --port, --preset, --selftest, --restore-proxy
// and --version. With --web it becomes the single-executable edition
// (DECISIONS.md D50): the same engine serves its own control page in
// the browser, so one file runs Younblock on Linux (and on Windows too,
// for anyone who prefers a browser tab to the desktop window).
package main

import (
        "context"
        "encoding/json"
        "flag"
        "fmt"
        "os"
        "os/exec"
        "os/signal"
        "syscall"
        "time"

        "github.com/assadigit/Younblock/internal/config"
        "github.com/assadigit/Younblock/internal/engine"
        "github.com/assadigit/Younblock/internal/health"
        "github.com/assadigit/Younblock/internal/platform"
        "github.com/assadigit/Younblock/internal/webui"
)

func main() {
        port := flag.Int("port", 0, "listen port (0 = use the saved setting)")
        preset := flag.String("preset", "", "fragmentation preset: light, balanced, strong or legacy")
        selftest := flag.Bool("selftest", false, "run the four-step connection test, print it and exit")
        restore := flag.Bool("restore-proxy", false, "restore the Windows internet settings and exit")
        web := flag.Bool("web", false, "serve the control UI in the browser (single-executable edition)")
        webPort := flag.Int("webport", 0, "port for the web UI (0 = the default port)")
        noOpen := flag.Bool("no-open", false, "with --web, do not try to open the browser automatically")
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

        if *web {
                runWebEdition(eng, *webPort, !*noOpen)
        } else {
                eng.Start()
        }

        sig := make(chan os.Signal, 1)
        signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
        fmt.Printf("%s core %s running. Press Ctrl+C to stop.\n", config.AppName, config.AppVersion)
        <-sig
        fmt.Println("Shutting down...")
        eng.Stop()
}

// runWebEdition starts the control-page server and, when the settings
// ask for it, connects like the desktop shell does on startup (the
// parity with app.go's startup connectOnLaunch handling).
func runWebEdition(eng *engine.Engine, webPort int, openBrowser bool) {
        if webPort == 0 {
                webPort = config.WebUIPort
        }
        s := webui.New(eng, webPort)
        if _, err := s.Listen(); err != nil {
                // The default port is taken (another copy, or an
                // unrelated program): take a free one instead of
                // failing, and say so plainly — the printed URL is the
                // address to open.
                fmt.Println("The default web port is busy; choosing a free one.")
                s = webui.New(eng, 0)
                if _, err := s.Listen(); err != nil {
                        fmt.Println("Web UI failed to start:", err)
                        os.Exit(1)
                }
        }
        go func() {
                if err := s.Serve(); err != nil {
                        fmt.Println("Web UI stopped:", err)
                }
        }()
        // Quit from the page ends the process (the engine has already
        // stopped and restored by then — QuitJSON's contract).
        eng.SetOnQuit(func() { os.Exit(0) })

        fmt.Printf("%s %s — control it at %s (Ctrl+C also stops it)\n",
                config.AppName, config.AppVersion, s.URL())
        if openBrowser {
                go func() {
                        // Best effort: on a headless box there is nothing
                        // to open and the printed URL is the answer.
                        _ = exec.Command("xdg-open", s.URL()).Run()
                }()
        }

        var settings struct {
                ConnectOnLaunch bool `json:"connectOnLaunch"`
        }
        if err := json.Unmarshal([]byte(eng.GetSettingsJSON()), &settings); err == nil && settings.ConnectOnLaunch {
                go eng.Start()
        }
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
