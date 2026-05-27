// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package intern manages a pool of interned values. An interned value is
// replaced by a small int.
//
// Go 1.23 introduced the [unique] package in the standard library with a
// similar purpose, but with different trade-offs:
//
//   - This package uses explicit reference counting (Put/Take) instead of
//     relying on the garbage collector and weak pointers for cleanup. This makes
//     memory reclamation deterministic rather than dependent on GC cycles.
//   - This package works with values that are not comparable. The [Value]
//     interface requires only Hash and Equal methods, while [unique.Make]
//     requires the comparable constraint.
//   - References are uint32 indices instead of pointers, resulting in a smaller
//     per-reference footprint (4 bytes vs 8 bytes on 64-bit platforms).
//   - This package uses explicit [Pool] instances rather than a single global
//     table, allowing separate namespaces and lifetimes. I believe this should
//     help efficiency as there are different maps of used values (but no
//     benchmark done).
//   - This package is not safe for concurrent use.
//   - This package has better performance (various benchmarks done on the bmp
//     package).
//
// [unique]: https://pkg.go.dev/unique
// [unique.Make]: https://pkg.go.dev/unique#Make
package intern

// Value is the interface that should be implemented by types
// used in an intern pool. Also, it should be immutable.
type Value[T any] interface {
	Hash() uint64
	Equal(T) bool
}

// Reference is a reference to an interned value. 0 is not a
// valid reference value.
type Reference[T any] uint32

// Pool keeps values in a pool by storing only one distinct copy
// of each. Values will be referred as an uint32 (implemented as an
// index).
type Pool[T Value[T]] struct {
	values           []internValue[T]
	availableIndexes []Reference[T]
	valueIndexes     map[uint64]Reference[T]
}

// internValue is the value stored in an intern pool. It adds resource
// keeping to the raw value.
type internValue[T Value[T]] struct {
	next     Reference[T] // next value with the same hash
	refCount uint32
	hash     uint64 // cached hash, avoids recomputing in Take

	value T
}

// NewPool creates a new intern pool.
func NewPool[T Value[T]]() *Pool[T] {
	return &Pool[T]{
		values:           make([]internValue[T], 1), // first slot is reserved
		availableIndexes: make([]Reference[T], 0),
		valueIndexes:     make(map[uint64]Reference[T]),
	}
}

// Get retrieves a (copy of the) value from the intern pool using its reference.
func (p *Pool[T]) Get(ref Reference[T]) T {
	return p.values[ref].value
}

// Take removes a value from the intern pool. If this is the last
// used reference, it will be deleted from the pool.
func (p *Pool[T]) Take(ref Reference[T]) {
	value := &p.values[ref]
	value.refCount--
	if value.refCount == 0 {
		p.availableIndexes = append(p.availableIndexes, ref)
		head := p.valueIndexes[value.hash]
		if head == ref {
			// We are the head of the chain
			if value.next > 0 {
				p.valueIndexes[value.hash] = value.next
			} else {
				delete(p.valueIndexes, value.hash)
			}
			return
		}
		// Walk the chain to find our predecessor. Only reached on hash
		// collisions, which are rare with a full uint64 hash.
		prev := head
		for p.values[prev].next != ref {
			prev = p.values[prev].next
		}
		p.values[prev].next = value.next
	}
}

// Ref returns the reference an interned value would have.
func (p *Pool[T]) Ref(value T) (Reference[T], bool) {
	hash := value.Hash()
	if index := p.valueIndexes[hash]; index > 0 {
		for index > 0 {
			if p.values[index].value.Equal(value) {
				return index, true
			}
			index = p.values[index].next
		}
	}
	return 0, false
}

// Put adds a value to the intern pool, returning its reference.
func (p *Pool[T]) Put(value T) Reference[T] {
	hash := value.Hash()
	v := internValue[T]{
		value:    value,
		refCount: 1,
		hash:     hash,
	}

	// Allocate a new index
	newIndex := func() Reference[T] {
		availCount := len(p.availableIndexes)
		if availCount > 0 {
			index := p.availableIndexes[availCount-1]
			p.availableIndexes = p.availableIndexes[:availCount-1]
			return index
		}
		if len(p.values) == cap(p.values) {
			// We need to extend capacity first
			temp := make([]internValue[T], len(p.values), (cap(p.values)+1)*2)
			copy(temp, p.values)
			p.values = temp
		}
		index := len(p.values)
		p.values = p.values[:index+1]
		return Reference[T](index)
	}

	// Check if we have already something
	if index := p.valueIndexes[hash]; index > 0 {
		prevIndex := index
		for index > 0 {
			if p.values[index].value.Equal(value) {
				p.values[index].refCount++
				return index
			}
			prevIndex = index
			index = p.values[index].next
		}

		// We have a collision, add to the chain
		index = newIndex()
		p.values[prevIndex].next = index
		p.values[index] = v
		return index
	}

	// Add a new one
	index := newIndex()
	p.values[index] = v
	p.valueIndexes[hash] = index
	return index
}

// Len returns the number of elements in the pool.
func (p *Pool[T]) Len() int {
	return len(p.values) - len(p.availableIndexes) - 1
}
