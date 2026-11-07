// SPDX-License-Identifier: AGPL-3.0-only

//go:build ignore

// In-memory Kafka broker used by bsd/smoke-test.sh, so that the orchestrator
// and the outlet can start without a real Kafka. Run with:
//
//	go run bsd/kfake.go [port]
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/twmb/franz-go/pkg/kfake"
)

func main() {
	port := 9092
	if len(os.Args) > 1 {
		p, err := strconv.Atoi(os.Args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid port:", err)
			os.Exit(1)
		}
		port = p
	}
	c, err := kfake.NewCluster(kfake.Ports(port), kfake.AllowAutoTopicCreation())
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot start kfake:", err)
		os.Exit(1)
	}
	defer c.Close()
	fmt.Println("kfake listening on", c.ListenAddrs())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}
