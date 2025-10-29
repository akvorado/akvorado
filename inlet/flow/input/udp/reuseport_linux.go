// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package udp

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

var (
	reuseportEBPFProgram *ebpf.Program
	reuseportEBPFMap     *ebpf.Map
	reuseportEBPFMu      sync.Mutex
)

// setupReuseportEBPF loads and attaches the eBPF program for SO_REUSEPORT load balancing.
func setupReuseportEBPF(fds []uintptr) error {
	var err error
	reuseportEBPFMu.Lock()
	defer reuseportEBPFMu.Unlock()
	cleanupReuseportEBPFUnlocked()

	reuseportEBPFProgram, reuseportEBPFMap, err = loadReuseportProgram(fds)
	if err != nil {
		return err
	}

	// Populate the map
	for i, fd := range fds {
		if err := reuseportEBPFMap.Put(uint32(i), uint64(fd)); err != nil {
			cleanupReuseportEBPFUnlocked()
			return fmt.Errorf("failed to update eBPF map: %w", err)
		}
	}

	// Assign the program to the first socket
	socketFD := int(fds[0])
	progFD := reuseportEBPFProgram.FD()
	if err := unix.SetsockoptInt(socketFD, unix.SOL_SOCKET, unix.SO_ATTACH_REUSEPORT_EBPF, progFD); err != nil {
		cleanupReuseportEBPFUnlocked()
		return fmt.Errorf("failed to attach eBPF program: %w", err)
	}
	return nil
}

// loadReuseportProgram loads the eBPF program for SO_REUSEPORT load balancing.
func loadReuseportProgram(fds []uintptr) (*ebpf.Program, *ebpf.Map, error) {
	reader := bytes.NewReader(bpfBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("can't load BPF program: %w", err)
	}
	numSockets := spec.Variables["num_sockets"]
	if numSockets == nil {
		return nil, nil, fmt.Errorf("can't locate eBPF variable %q", "num_sockets")
	}
	if err := numSockets.Set(uint32(len(fds))); err != nil {
		return nil, nil, fmt.Errorf("can't set eBPF variable %q: %w", "num_sockets", err)
	}
	assignment := struct {
		Program   *ebpf.Program `ebpf:"reuseport_balance_prog"`
		SocketMap *ebpf.Map     `ebpf:"socket_map"`
	}{}
	if err := spec.LoadAndAssign(&assignment, nil); err != nil {
		// Check if the error is "operation not permitted" to provide a clearer message
		if strings.Contains(err.Error(), "operation not permitted") {
			err = errors.New("operation not permitted (BPF capability missing or MEMLOCK too low)")
		}
		return nil, nil, fmt.Errorf("can't assign eBPF programs: %w", err)
	}
	return assignment.Program, assignment.SocketMap, nil
}

func cleanupReuseportEBPF() {
	reuseportEBPFMu.Lock()
	defer reuseportEBPFMu.Unlock()
	cleanupReuseportEBPFUnlocked()
}

func cleanupReuseportEBPFUnlocked() {
	if reuseportEBPFProgram != nil {
		reuseportEBPFProgram.Close()
		reuseportEBPFProgram = nil
	}
	if reuseportEBPFMap != nil {
		reuseportEBPFMap.Close()
		reuseportEBPFMap = nil
	}
}
