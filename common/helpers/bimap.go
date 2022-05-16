package helpers

import "fmt"

// Bimap is a bidirectional map.
type Bimap[K, V comparable] struct {
	forward map[K]V
	inverse map[V]K
}

// NewBimap returns a new bimap from an existing map.
func NewBimap[K, V comparable](input map[K]V) *Bimap[K, V] {
	output := &Bimap[K, V]{
		forward: make(map[K]V),
		inverse: make(map[V]K),
	}
	for key, value := range input {
		output.forward[key] = value
		output.inverse[value] = key
	}
	return output
}

// LoadValue returns the value stored in the bimap for a key.
func (bi *Bimap[K, V]) LoadValue(k K) (V, bool) {
	v, ok := bi.forward[k]
	return v, ok
}

// LoadKey returns the key stored in the bimap for a value.
func (bi *Bimap[K, V]) LoadKey(v V) (K, bool) {
	k, ok := bi.inverse[v]
	return k, ok
}

// Keys returns a slice of the keys in the bimap.
func (bi *Bimap[K, V]) Keys() []K {
	var keys []K
	for k := range bi.forward {
		keys = append(keys, k)
	}
	return keys
}

// Values returns a slice of the values in the bimap.
func (bi *Bimap[K, V]) Values() []V {
	var values []V
	for v := range bi.inverse {
		values = append(values, v)
	}
	return values
}

// String returns a string representation of the bimap.
func (bi *Bimap[K, V]) String() string {
	return fmt.Sprintf("Bi%v", bi.forward)
}
