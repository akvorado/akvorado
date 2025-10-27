// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build ignore

#include "vmlinux.h"

volatile const __u32 num_sockets;

struct {
    __uint(type, BPF_MAP_TYPE_REUSEPORT_SOCKARRAY);
    __type(key, __u32);
    __type(value, __u64);
    __uint(max_entries, 256);
} socket_map SEC(".maps");

// SO_REUSEPORT program to distribute incoming packets across workers. This
// program is called for each incoming packet and returns the socket index to
// which the packet should be delivered.
SEC("sk_reuseport")
int reuseport_balance_prog(struct sk_reuseport_md *reuse_md)
{
    __u32 index = bpf_get_prandom_u32() % num_sockets;
    bpf_sk_select_reuseport(reuse_md, &socket_map, &index, 0);
    return SK_PASS;
}

char _license[] SEC("license") = "GPL";
