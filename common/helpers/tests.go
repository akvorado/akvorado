// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

// Package helpers contains small functions usable by any other
// package, both for testing or not.
package helpers

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

// CheckExternalService checks an external service, available either
// as a named service or on a specific port on localhost. This applies
// for example for Kafka and ClickHouse. The timeouts are quite short,
// but we suppose that either the services are run through
// docker compose manually and ready, either through CI and they are
// checked for readiness.
func CheckExternalService(t *testing.T, name string, dnsCandidates []string, port string) string {
	t.Helper()
	if testing.Short() {
		t.Skipf("Skip test with real %s in short mode", name)
	}
	mandatory := os.Getenv("CI_AKVORADO_FUNCTIONAL_TESTS") != ""
	var err error

	found := ""
	for _, dnsCandidate := range dnsCandidates {
		resolv := net.Resolver{PreferGo: true}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, err = resolv.LookupHost(ctx, dnsCandidate)
		cancel()
		if err == nil {
			found = dnsCandidate
			break
		}
	}
	if found == "" {
		if mandatory {
			t.Fatalf("%s cannot be resolved (CI_AKVORADO_FUNCTIONAL_TESTS is set)", name)
		}
		t.Skipf("%s cannot be resolved (CI_AKVORADO_FUNCTIONAL_TESTS is not set)", name)
	}

	var d net.Dialer
	server := net.JoinHostPort(found, port)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	for {
		_, err := d.DialContext(ctx, "tcp", server)
		if err == nil {
			break
		}
		if mandatory {
			t.Logf("DialContext() error:\n%+v", err)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			if mandatory {
				t.Fatalf("%s is not running (CI_AKVORADO_FUNCTIONAL_TESTS is set)", name)
			} else {
				t.Skipf("%s is not running (CI_AKVORADO_FUNCTIONAL_TESTS is not set)", name)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()

	return server
}

// StartStop starts a component and stops it on cleanup.
func StartStop(t *testing.T, component interface{}) {
	t.Helper()
	if starterC, ok := component.(starter); ok {
		if err := starterC.Start(); err != nil {
			t.Fatalf("Start() error:\n%+v", err)
		}
	}
	t.Cleanup(func() {
		if stopperC, ok := component.(stopper); ok {
			if err := stopperC.Stop(); err != nil {
				t.Errorf("Stop() error:\n%+v", err)
			}
		}
	})
}

type starter interface {
	Start() error
}
type stopper interface {
	Stop() error
}
