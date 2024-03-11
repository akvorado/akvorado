// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kentik/patricia"
	tree "github.com/kentik/patricia/generics_tree"
	"github.com/mitchellh/mapstructure"
)

// SubnetMap maps subnets to values and allow to lookup by IP address.
// Internally, everything is stored as an IPv6 (using v6-mapped IPv4
// addresses).
type SubnetMap[V any] struct {
	tree *tree.TreeV6[V]
}

// Lookup will search for the most specific subnet matching the
// provided IP address and return the value associated with it.
func (sm *SubnetMap[V]) Lookup(ip netip.Addr) (V, bool) {
	if sm == nil || sm.tree == nil {
		var value V
		return value, false
	}
	ok, value := sm.tree.FindDeepestTag(patricia.NewIPv6Address(ip.AsSlice(), 128))
	return value, ok
}

// LookupOrDefault calls lookup and if not found, will return the
// provided default value.
func (sm *SubnetMap[V]) LookupOrDefault(ip netip.Addr, fallback V) V {
	if value, ok := sm.Lookup(ip); ok {
		return value
	}
	return fallback
}

// ToMap return a map of the tree.
func (sm *SubnetMap[V]) ToMap() map[string]V {
	output := map[string]V{}
	if sm == nil || sm.tree == nil {
		return output
	}
	iter := sm.tree.Iterate()
	for iter.Next() {
		output[iter.Address().String()] = iter.Tags()[0]
	}
	return output
}

// Set inserts the given key k into the SubnetMap, replacing any existing value if it exists.
func (sm *SubnetMap[V]) Set(k string, v V) error {
	subnetK, err := SubnetMapParseKey(k)
	if err != nil {
		return err
	}
	_, ipNet, err := net.ParseCIDR(subnetK)
	if err != nil {
		// Should not happen
		return err
	}
	_, bits := ipNet.Mask.Size()
	if bits != 128 {
		return fmt.Errorf("%q is not an IPv6 subnet", ipNet)
	}
	plen, _ := ipNet.Mask.Size()
	sm.tree.Set(patricia.NewIPv6Address(ipNet.IP.To16(), uint(plen)), v)
	return nil
}

// Update inserts the given key k into the SubnetMap, calling updateFunc with the existing value.
func (sm *SubnetMap[V]) Update(k string, v V, updateFunc tree.UpdatesFunc[V]) error {
	subnetK, err := SubnetMapParseKey(k)
	if err != nil {
		return err
	}
	_, ipNet, err := net.ParseCIDR(subnetK)
	if err != nil {
		// Should not happen
		return err
	}
	_, bits := ipNet.Mask.Size()
	if bits != 128 {
		return fmt.Errorf("%q is not an IPv6 subnet", ipNet)
	}
	plen, _ := ipNet.Mask.Size()
	sm.tree.SetOrUpdate(patricia.NewIPv6Address(ipNet.IP.To16(), uint(plen)), v, updateFunc)
	return nil
}

// Iter enables iteration of the SubnetMap, calling f for every entry. If f returns an error, the iteration is aborted.
func (sm *SubnetMap[V]) Iter(f func(address patricia.IPv6Address, tags [][]V) error) error {
	iter := sm.tree.Iterate()
	for iter.Next() {
		if err := f(iter.Address(), iter.TagsFromRoot()); err != nil {
			return err
		}
	}
	return nil
}

// NewSubnetMap creates a subnetmap from a map. Unlike user-provided
// configuration, this function is stricter and require everything to
// be IPv6 subnets.
func NewSubnetMap[V any](from map[string]V) (*SubnetMap[V], error) {
	trie := &SubnetMap[V]{tree.NewTreeV6[V]()}
	if from == nil {
		return trie, nil
	}
	for k, v := range from {
		trie.Set(k, v)
	}
	return trie, nil
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

var subnetLookAlikeRegex = regexp.MustCompile("^([a-fA-F:.0-9]+)(/([0-9]+))?$")

// SubnetMapUnmarshallerHook decodes SubnetMap and notably check that
// valid networks are provided as key. It also accepts a single value
// instead of a map for backward compatibility.
func SubnetMapUnmarshallerHook[V any]() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if to.Type() != reflect.TypeOf(SubnetMap[V]{}) {
			return from.Interface(), nil
		}
		if from.Type() == reflect.TypeOf(&SubnetMap[V]{}) {
			return from.Interface(), nil
		}
		output := gin.H{}
		var zero V
		var plausibleSubnetMap bool
		if from.Kind() == reflect.Map {
			// When we have a map, we check if all keys look like a subnet.
			plausibleSubnetMap = true
			for _, key := range from.MapKeys() {
				key = ElemOrIdentity(key)
				if key.Kind() != reflect.String {
					plausibleSubnetMap = false
					break
				}
				if !subnetLookAlikeRegex.MatchString(key.String()) {
					plausibleSubnetMap = false
					break
				}
			}
		}
		if plausibleSubnetMap {
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
				output[key] = v.Interface()
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

// SubnetMapParseKey decodes and validates a key used in SubnetMap from a network string.
func SubnetMapParseKey(k string) (string, error) {
	var key string
	if strings.Contains(k, "/") {
		// Subnet
		_, ipNet, err := net.ParseCIDR(k)
		if err != nil {
			return "", err
		}
		// Convert key to IPv6
		ones, bits := ipNet.Mask.Size()
		if bits != 32 && bits != 128 {
			return "", fmt.Errorf("key %s has invalid netmask", k)
		}
		if bits == 32 {
			key = fmt.Sprintf("::ffff:%s/%d", ipNet.IP.String(), ones+96)
		} else if ipNet.IP.To4() != nil {
			key = fmt.Sprintf("::ffff:%s/%d", ipNet.IP.String(), ones)
		} else {
			key = ipNet.String()
		}
	} else {
		// IP
		ip := net.ParseIP(k)
		if ip == nil {
			return "", fmt.Errorf("key %s is not a valid subnet", k)
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			key = fmt.Sprintf("::ffff:%s/128", ipv4.String())
		} else {
			key = fmt.Sprintf("%s/128", ip.String())
		}
	}
	return key, nil
}

// MarshalYAML turns a subnet into a map that can be marshaled.
func (sm SubnetMap[V]) MarshalYAML() (interface{}, error) {
	return sm.ToMap(), nil
}

func (sm SubnetMap[V]) String() string {
	out := sm.ToMap()
	return fmt.Sprintf("%+v", out)
}
