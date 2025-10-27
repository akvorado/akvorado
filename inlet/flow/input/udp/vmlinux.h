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
    /* When reuse->migrating_sk is NULL, it is selecting a sk for the
     * new incoming connection request (e.g. selecting a listen sk for
     * the received SYN in the TCP case).  reuse->sk is one of the sk
     * in the reuseport group. The bpf prog can use reuse->sk to learn
     * the local listening ip/port without looking into the skb.
     *
     * When reuse->migrating_sk is not NULL, reuse->sk is closed and
     * reuse->migrating_sk is the socket that needs to be migrated
     * to another listening socket.  migrating_sk could be a fullsock
     * sk that is fully established or a reqsk that is in-the-middle
     * of 3-way handshake.
     */
    __bpf_md_ptr(struct bpf_sock *, sk);
    __bpf_md_ptr(struct bpf_sock *, migrating_sk);
};

/* bpf_get_prandom_u32
 *
 * 	Get a pseudo-random number.
 *
 * 	From a security point of view, this helper uses its own
 * 	pseudo-random internal state, and cannot be used to infer the
 * 	seed of other random functions in the kernel. However, it is
 * 	essential to note that the generator used by the helper is not
 * 	cryptographically secure.
 *
 * Returns
 * 	A random 32-bit unsigned value.
 */
static __u32 (*bpf_get_prandom_u32)(void) = (void *) 7;

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
