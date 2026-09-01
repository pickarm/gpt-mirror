package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"PandoraHelper/internal/webmirror"
)

func main() {
	upstream := flag.String("upstream", "https://chatgpt.com/", "upstream page URL to inspect")
	mirror := flag.String("mirror", "https://mirror.example.com/", "candidate mirror origin")
	timeout := flag.Duration("timeout", 20*time.Second, "probe timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	report, err := webmirror.Probe(ctx, &http.Client{Timeout: *timeout}, *upstream, *mirror)
	if err != nil {
		fmt.Fprintln(os.Stderr, "probe failed:", err)
		os.Exit(1)
	}
	encoded, err := report.JSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode report:", err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
