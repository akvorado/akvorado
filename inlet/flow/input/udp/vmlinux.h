// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef __VMLINUX_H__
#define __VMLINUX_H__

typedef unsigned int __u32;
typedef long long unsigned int __u64;

enum sk_action {
    SK_DROP = 0,
    SK_PASS = 1,
};

enum bpf_map_type {
	BPF_MAP_TYPE_PERCPU_ARRAY          = 6,
	BPF_MAP_TYPE_REUSEPORT_SOCKARRAY   = 20,
};

#define __uint(name, val) int (*name)[val]
#define __type(name, val) typeof(val) *name

#define __bpf_md_ptr(type, name)                \
    union {                                     \
        type name;                              \
    __u64 :64;                                  \
    } __attribute__((aligned(8)))

struct sk_reuseport_md {
    /*
     * Start of directly accessible data. It begins from
     * the tcp/udp header.
     */
    __bpf_md_ptr(void *, data);
    /* End of directly accessible data */
    __bpf_md_ptr(void *, data_end);
    /*
     * Total length of packet (starting from the tcp/udp header).
     * Note that the directly accessible bytes (data_end - data)
     * could be less than this "len".  Those bytes could be
     * indirectly read by a helper "bpf_skb_load_bytes()".
     */
    __u32 len;
    /*
     * Eth protocol in the mac header (network byte order). e.g.
     * ETH_P_IP(0x0800) and ETH_P_IPV6(0x86DD)
     */
    __u32 eth_protocol;
    __u32 ip_protocol;	/* IP protocol. e.g. IPPROTO_TCP, IPPROTO_UDP */
    __u32 bind_inany;	/* Is sock bound to an INANY address? */
    __u32 hash;		/* A hash of the packet 4 tuples */
} __attribute__((preserve_access_index));

/*
 * bpf_map_lookup_elem
 *
 * 	Perform a lookup in *map* for an entry associated to *key*.
 *
 * Returns
 * 	Map value associated to *key*, or **NULL** if no entry was
 * 	found.
 */
static void *(*bpf_map_lookup_elem)(void *map, const void *key) = (void *) 1;

/*
 * bpf_sk_select_reuseport
 *
 * 	Select a **SO_REUSEPORT** socket from a
 * 	**BPF_MAP_TYPE_REUSEPORT_SOCKARRAY** *map*.
 * 	It checks the selected socket is matching the incoming
 * 	request in the socket buffer.
 *
 * Returns
 * 	0 on success, or a negative error in case of failure.
 */
static long (*bpf_sk_select_reuseport)(struct sk_reuseport_md *reuse, void *map, void *key, __u64 flags) = (void *) 82;

#define SEC(name)                                               \
    _Pragma("GCC diagnostic push")                              \
    _Pragma("GCC diagnostic ignored \"-Wignored-attributes\"")  \
    __attribute__((section(name), used))                        \
    _Pragma("GCC diagnostic pop")                               \

#endif
