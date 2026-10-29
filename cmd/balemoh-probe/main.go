// balemoh-probe checks loopback listeners in containers without a shell.
package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: balemoh-probe http://127.0.0.1:PORT/healthz")
		os.Exit(1)
	}
	target, err := url.Parse(os.Args[1])
	if err != nil || target.Scheme != "http" || target.Path != "/healthz" || target.User != nil || !net.ParseIP(target.Hostname()).IsLoopback() {
		fmt.Fprintln(os.Stderr, "probe requires a loopback HTTP health endpoint")
		os.Exit(1)
	}
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(target.String())
	if err != nil {
		fmt.Fprintln(os.Stderr, "health endpoint unavailable")
		os.Exit(1)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "health endpoint unhealthy")
		os.Exit(1)
	}
}
