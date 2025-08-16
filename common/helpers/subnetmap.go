// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"fmt"
	"iter"
	"net/netip"
	"reflect"
	"regexp"
	"strings"

	"github.com/gaissmai/bart"
	"github.com/gin-gonic/gin"
	"github.com/go-viper/mapstructure/v2"
)

// SubnetMap maps subnets to values and allow to lookup by IP address.
type SubnetMap[V any] struct {
	table *bart.Table[V]
}

// Lookup will search for the most specific subnet matching the
// provided IP address and return the value associated with it.
func (sm *SubnetMap[V]) Lookup(ip netip.Addr) (V, bool) {
	if sm == nil || sm.table == nil {
		var value V
		return value, false
	}
	return sm.table.Lookup(ip)
}

// LookupOrDefault calls lookup and if not found, will return the
// provided default value.
func (sm *SubnetMap[V]) LookupOrDefault(ip netip.Addr, fallback V) V {
	if value, ok := sm.Lookup(ip); ok {
		return value
	}
	return fallback
}

// ToMap return a map of the tree. This should be used only when handling user
// configuration or for debugging. Otherwise, it is better to use Iter().
func (sm *SubnetMap[V]) ToMap() map[string]V {
	output := map[string]V{}
	for prefix, value := range sm.All() {
		if prefix.Addr().Is4In6() {
			ipv4Addr := prefix.Addr().Unmap()
			ipv4Prefix := netip.PrefixFrom(ipv4Addr, prefix.Bits()-96)
			output[ipv4Prefix.String()] = value
			continue
		}
		output[prefix.String()] = value
	}
	return output
}

// Set inserts the given key k into the SubnetMap, replacing any existing value
// if it exists. It requires an IPv6 prefix or it will panic.
func (sm *SubnetMap[V]) Set(prefix netip.Prefix, v V) {
	if !prefix.Addr().Is6() {
		panic(fmt.Errorf("%q is not an IPv6 subnet", prefix))
	}
	if sm.table == nil {
		sm.table = &bart.Table[V]{}
	}
	sm.table.Insert(prefix, v)
}

// Update inserts the given key k into the SubnetMap, calling cb with the
// existing value. It requires an IPv6 prefix or it will panic.
func (sm *SubnetMap[V]) Update(prefix netip.Prefix, cb func(V, bool) V) {
	if !prefix.Addr().Is6() {
		panic(fmt.Errorf("%q is not an IPv6 subnet", prefix))
	}
	if sm.table == nil {
		sm.table = &bart.Table[V]{}
	}
	sm.table.Update(prefix, cb)
}

// All walks the whole subnet map.
func (sm *SubnetMap[V]) All() iter.Seq2[netip.Prefix, V] {
	return func(yield func(netip.Prefix, V) bool) {
		if sm == nil || sm.table == nil {
			return
		}
		sm.table.All6()(yield)
	}
}

// AllMaybeSorted walks the whole subnet map in sorted order during tests but
// not when running tests.
func (sm *SubnetMap[V]) AllMaybeSorted() iter.Seq2[netip.Prefix, V] {
	return func(yield func(netip.Prefix, V) bool) {
		if sm == nil || sm.table == nil {
			return
		}
		if Testing() {
			sm.table.AllSorted6()(yield)
		} else {
			sm.table.All6()(yield)
		}
	}
}

// Supernets returns an iterator over all supernet routes that cover the given
// prefix. The iteration order is reverse-CIDR: from longest prefix match (LPM)
// towards least-specific routes.
func (sm *SubnetMap[V]) Supernets(prefix netip.Prefix) iter.Seq2[netip.Prefix, V] {
	return func(yield func(netip.Prefix, V) bool) {
		if sm == nil || sm.table == nil {
			return
		}
		sm.table.Supernets(prefix)(yield)
	}
}

// NewSubnetMap creates a subnetmap from a map. It should not be used in a hot
// path as it builds the subnet from a map keyed by strings.
func NewSubnetMap[V any](from map[string]V) (*SubnetMap[V], error) {
	sm := &SubnetMap[V]{table: &bart.Table[V]{}}
	if from == nil {
		return sm, nil
	}
	for k, v := range from {
		key, err := SubnetMapParseKey(k)
		if err != nil {
			return nil, fmt.Errorf("failed to parse key %s: %w", k, err)
		}
		sm.Set(key, v)
	}
	return sm, nil
}

// MustNewSubnetMap creates a subnet from a map and panic in case of a
// problem. This should only be used with tests.
func MustNewSubnetMap[V any](from map[string]V) *SubnetMap[V] {
	trie, err := NewSubnetMap(from)
	if err != nil {
		panic(err)
	}
	return trie
}

// subnetLookAlikeRegex is a regex that matches string looking like a subnet,
// allowing better error messages if there is a typo.
var subnetLookAlikeRegex = regexp.MustCompile("^([a-fA-F:.0-9]*[:.][a-fA-F:.0-9]*)(/([0-9]+))?$")

// LooksLikeSubnetMap returns true iff the provided value could be a SubnetMap
// (but not 100% sure).
func LooksLikeSubnetMap(v reflect.Value) (result bool) {
	if v.Kind() == reflect.Map {
		// When we have a map, we check if all keys look like a subnet.
		result = true
		for _, key := range v.MapKeys() {
			key = ElemOrIdentity(key)
			if key.Kind() != reflect.String {
				result = false
				break
			}
			if !subnetLookAlikeRegex.MatchString(key.String()) {
				result = false
				break
			}
		}
	}
	return
}

// SubnetMapUnmarshallerHook decodes SubnetMap and notably check that valid
// networks are provided as key. It also accepts a single value instead of a map
// for backward compatibility. It should not be used in hot paths as it builds
// an intermediate map.
func SubnetMapUnmarshallerHook[V any]() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (any, error) {
		if to.Type() != reflect.TypeOf(SubnetMap[V]{}) {
			return from.Interface(), nil
		}
		if from.Type() == reflect.TypeOf(&SubnetMap[V]{}) {
			return from.Interface(), nil
		}
		output := gin.H{}
		var zero V
		if LooksLikeSubnetMap(from) {
			// First case, we have a map
			iter := from.MapRange()
			for i := 0; iter.Next(); i++ {
				k := ElemOrIdentity(iter.Key())
				v := iter.Value()
				if k.Kind() != reflect.String {
					return nil, fmt.Errorf("key %d is not a string (%s)", i, k.Kind())
				}
				// Parse key
				key, err := SubnetMapParseKey(k.String())
				if err != nil {
					return nil, fmt.Errorf("failed to parse key %s: %w", key, err)
				}
				output[key.String()] = v.Interface()
			}
		} else {
			// Second case, we have a single value and we let mapstructure handles it
			output["::/0"] = from.Interface()
		}

		// We have to decode output map, then turn it into a SubnetMap[V]
		var intermediate map[string]V
		intermediateDecoder, err := mapstructure.NewDecoder(
			GetMapStructureDecoderConfig(&intermediate))
		if err != nil {
			return nil, fmt.Errorf("cannot create subdecoder: %w", err)
		}
		if err := intermediateDecoder.Decode(output); err != nil {
			return nil, fmt.Errorf("unable to decode %q: %w", reflect.TypeOf(zero).Name(), err)
		}
		trie, err := NewSubnetMap(intermediate)
		if err != nil {
			// Should not happen
			return nil, err
		}

		return trie, nil
	}
}

// PrefixTo16 converts an IPv4 prefix to an IPv4-mapped IPv6 prefix.
// IPv6 prefixes are returned as-is.
func PrefixTo16(prefix netip.Prefix) netip.Prefix {
	if prefix.Addr().Is6() {
		return prefix
	}
	// Convert IPv4 to IPv4-mapped IPv6
	return netip.PrefixFrom(netip.AddrFrom16(prefix.Addr().As16()), prefix.Bits()+96)
}

// SubnetMapParseKey parses a prefix or an IP address into a netip.Prefix that
// can be used in a map.
func SubnetMapParseKey(k string) (netip.Prefix, error) {
	// Subnet
	if strings.Contains(k, "/") {
		key, err := netip.ParsePrefix(k)
		if err != nil {
			return netip.Prefix{}, err
		}
		return PrefixTo16(key), nil
	}
	// IP address
	key, err := netip.ParseAddr(k)
	if err != nil {
		return netip.Prefix{}, err
	}
	if key.Is4() {
		return PrefixTo16(netip.PrefixFrom(key, 32)), nil
	}
	return netip.PrefixFrom(key, 128), nil
}

// MarshalYAML turns a subnet into a map that can be marshaled.
func (sm SubnetMap[V]) MarshalYAML() (any, error) {
	return sm.ToMap(), nil
}

func (sm SubnetMap[V]) String() string {
	out := sm.ToMap()
	return fmt.Sprintf("%+v", out)
}
