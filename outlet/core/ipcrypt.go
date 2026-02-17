// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"errors"
	"net"

	ipcrypt "github.com/jedisct1/go-ipcrypt"
)

// IPcrypt is a thin wrapper around the upstream go-ipcrypt implementation.
type IPcrypt struct {
	key []byte
}

// NewIPcrypt constructs an IPcrypt wrapper. Accepts 16 or 32 byte keys.
func NewIPcrypt(key []byte) (*IPcrypt, error) {
	if len(key) != ipcrypt.KeySizeDeterministic && len(key) != ipcrypt.KeySizeND && len(key) != ipcrypt.KeySizeNDX {
		return nil, errors.New("ipcrypt: key must be 16 or 32 bytes")
	}
	return &IPcrypt{key: append([]byte(nil), key...)}, nil
}

// AnonymizeIPv4 returns an anonymized IPv4 address or nil if ip is not IPv4.
func (c *IPcrypt) AnonymizeIPv4(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	if ip.To4() == nil {
		return nil
	}
	out, err := ipcrypt.EncryptIPPfx(ip, c.key)
	if err != nil {
		return nil
	}
	if v4 := out.To4(); v4 != nil {
		return append(net.IP(nil), v4...)
	}
	return append(net.IP(nil), out...)
}

// AnonymizeIPv6 returns an anonymized IPv6 address or nil if ip is not IPv6.
func (c *IPcrypt) AnonymizeIPv6(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	if ip.To4() != nil {
		return nil
	}
	out, err := ipcrypt.EncryptIPPfx(ip, c.key)
	if err != nil {
		return nil
	}
	if v6 := out.To16(); v6 != nil {
		return append(net.IP(nil), v6...)
	}
	return append(net.IP(nil), out...)
}
