// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

// InternValue is the interface that should be implemented by types
// used in an intern pool. Also, it should be immutable.
type InternValue[T any] interface {
	Hash() uint64
	Equal(T) bool
}

// InternReference is a reference to an interned value. 0 is not a
// valid reference value.
type InternReference[T any] uint32

// InternPool keeps values in a pool by storing only one distinct copy
// of each. Values will be referred as an uint32 (implemented as an
// index).
type InternPool[T InternValue[T]] struct {
	values           []internValue[T]
	availableIndexes []InternReference[T]
	valueIndexes     map[uint64]InternReference[T]
}

// internValue is the value stored in an intern pool. It adds resource
// keeping to the raw value.
type internValue[T InternValue[T]] struct {
	next     InternReference[T] // next value with the same hash
	previous InternReference[T] // previous value with the same hash
	refCount uint32

	value T
}

// NewInternPool creates a new intern pool.
func NewInternPool[T InternValue[T]]() *InternPool[T] {
	return &InternPool[T]{
		values:           make([]internValue[T], 1), // first slot is reserved
		availableIndexes: make([]InternReference[T], 0),
		valueIndexes:     make(map[uint64]InternReference[T]),
	}
}

// Get retrieves a (copy of the) value from the intern pool using its reference.
func (p *InternPool[T]) Get(ref InternReference[T]) T {
	return p.values[ref].value
}

// Take removes a value from the intern pool. If this is the last
// used reference, it will be deleted from the pool.
func (p *InternPool[T]) Take(ref InternReference[T]) {
	value := &p.values[ref]
	value.refCount--
	if value.refCount == 0 {
		p.availableIndexes = append(p.availableIndexes, ref)
		if value.previous > 0 {
			// Not the first one, link previous to next
			p.values[value.previous].next = value.next
			p.values[value.next].previous = value.previous
			return
		}
		hash := value.value.Hash()
		if value.next > 0 {
			// We are the first one of a chain, move the pointer to the next one
			p.valueIndexes[hash] = value.next
			p.values[value.next].previous = 0
			return
		}
		// Last case, we are the last one, let's find our hash and delete us from here
		delete(p.valueIndexes, hash)
	}
}

// Put adds a value to the intern pool, returning its reference.
func (p *InternPool[T]) Put(value T) InternReference[T] {
	v := internValue[T]{
		value:    value,
		refCount: 1,
		previous: 0,
		next:     0,
	}

	// Allocate a new index
	newIndex := func() InternReference[T] {
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
		return InternReference[T](index)
	}

	// Check if we have already something
	hash := value.Hash()
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
		v.previous = prevIndex
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
func (p *InternPool[T]) Len() int {
	return len(p.values) - len(p.availableIndexes) - 1
}
