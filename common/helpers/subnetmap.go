// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"fmt"
	"net"
	"reflect"
	"regexp"
	"strings"

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
func (sm *SubnetMap[V]) Lookup(ip net.IP) (V, bool) {
	if sm.tree == nil {
		var value V
		return value, false
	}
	ip = ip.To16()
	ok, value := sm.tree.FindDeepestTag(patricia.NewIPv6Address(ip, 128))
	return value, ok
}

// LookupOrDefault calls lookup and if not found, will return the
// provided default value.
func (sm *SubnetMap[V]) LookupOrDefault(ip net.IP, fallback V) V {
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

// NewSubnetMap creates a subnetmap from a map. Unlike user-provided
// configuration, this function is stricter and require everything to
// be IPv6 subnets.
func NewSubnetMap[V any](from map[string]V) (*SubnetMap[V], error) {
	trie := tree.NewTreeV6[V]()
	for k, v := range from {
		_, ipNet, err := net.ParseCIDR(k)
		if err != nil {
			// Should not happen
			return nil, err
		}
		_, bits := ipNet.Mask.Size()
		if bits != 128 {
			return nil, fmt.Errorf("%q is not an IPv6 subnet", ipNet)
		}
		plen, _ := ipNet.Mask.Size()
		trie.Set(patricia.NewIPv6Address(ipNet.IP.To16(), uint(plen)), v)
	}
	return &SubnetMap[V]{trie}, nil
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
		output := map[string]interface{}{}
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
				var key string
				if strings.Contains(k.String(), "/") {
					// Subnet
					_, ipNet, err := net.ParseCIDR(k.String())
					if err != nil {
						return nil, err
					}
					// Convert key to IPv6
					ones, bits := ipNet.Mask.Size()
					if bits != 32 && bits != 128 {
						return nil, fmt.Errorf("key %d has an invalid netmask", i)
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
					ip := net.ParseIP(k.String())
					if ip == nil {
						return nil, fmt.Errorf("key %d is not a valid subnet", i)
					}
					if ipv4 := ip.To4(); ipv4 != nil {
						key = fmt.Sprintf("::ffff:%s/128", ipv4.String())
					} else {
						key = fmt.Sprintf("%s/128", ip.String())
					}
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

// MarshalYAML turns a subnet into a map that can be marshaled.
func (sm SubnetMap[V]) MarshalYAML() (interface{}, error) {
	return sm.ToMap(), nil
}

func (sm SubnetMap[V]) String() string {
	out := sm.ToMap()
	return fmt.Sprintf("%+v", out)
}
