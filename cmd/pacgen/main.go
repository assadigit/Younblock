// Command pacgen prints the generated PAC script to stdout so the Node.js
// PAC behavior test (scripts/pac-test.mjs) can evaluate the exact text the
// proxy serves. Usage: pacgen <mode> [port].
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/assadigit/Younblock/internal/config"
	"github.com/assadigit/Younblock/internal/proxy"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: pacgen <youtube|all|manual> [port]")
		os.Exit(2)
	}
	mode := config.Mode(os.Args[1])
	port := 4525
	if len(os.Args) > 2 {
		p, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad port:", err)
			os.Exit(2)
		}
		port = p
	}
	fmt.Print(proxy.PAC(port, config.DefaultDomains(), mode))
}
