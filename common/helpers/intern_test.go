// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import "testing"

type likeInt int

func (i likeInt) Equal(j likeInt) bool { return i == j }
func (i likeInt) Hash() uint64         { return uint64(i) % 10 }

func TestPut(t *testing.T) {
	p := NewInternPool[likeInt]()

	a := p.Put(likeInt(10))
	b := p.Put(likeInt(10))
	c := p.Put(likeInt(11))
	d := p.Put(likeInt(12))

	if a != b {
		t.Error("got two references for Put(10)")
	}
	if a == c || a == d || c == d {
		t.Error("got same reference for Put(10)/Put(11)/Put(12)")
	}
	if p.Get(a) != likeInt(10) {
		t.Errorf("Get(Put(10)) == %d != 10", p.Get(a))
	}
	if p.Get(c) != likeInt(11) {
		t.Errorf("Get(Put(11)) == %d != 10", p.Get(c))
	}
	if p.Get(d) != likeInt(12) {
		t.Errorf("Get(Put(12)) == %d != 10", p.Get(d))
	}
}

func TestRef(t *testing.T) {
	p := NewInternPool[likeInt]()
	a := p.Put(likeInt(10))
	b, bOK := p.Ref(likeInt(10))
	c, cOK := p.Ref(likeInt(20))
	if a != b {
		t.Error("got two references for Put/Ref(10)")
	}
	if !bOK {
		t.Error("didn't get a ref for Ref(10)")
	}
	if cOK {
		t.Error("got a ref for Ref(20)")
	}
	if c != 0 {
		t.Error("got a value for Ref(20)")
	}
}

func TestPutCollision(t *testing.T) {
	p := NewInternPool[likeInt]()

	a := p.Put(likeInt(10))
	b := p.Put(likeInt(20))
	c := p.Put(likeInt(11))
	d := p.Put(likeInt(21))
	if a == b || a == c || a == d || b == c || b == d || c == d {
		t.Error("got same reference for two different values")
	}
}

func TestTake(t *testing.T) {
	p := NewInternPool[likeInt]()

	val1 := likeInt(10)
	ref1 := p.Put(val1)
	val2 := likeInt(10)
	ref2 := p.Put(val2)
	val3 := likeInt(12)
	ref3 := p.Put(val3)
	val4 := likeInt(22) // collision
	ref4 := p.Put(val4)
	val5 := likeInt(32)
	ref5 := p.Put(val5)

	expectedValues := []internValue[likeInt]{
		{},
		{value: 10, refCount: 2},
		{value: 12, refCount: 1, next: 3},
		{value: 22, refCount: 1, previous: 2, next: 4},
		{value: 32, refCount: 1, previous: 3},
	}
	if diff := Diff(p.values, expectedValues, DiffUnexported); diff != "" {
		t.Fatalf("p.values (-got, +want):\n%s", diff)
	}

	p.Take(ref4)

	expectedValues = []internValue[likeInt]{
		{},
		{value: 10, refCount: 2},
		{value: 12, refCount: 1, next: 4},
		{value: 22, refCount: 0, previous: 2, next: 4}, // free
		{value: 32, refCount: 1, previous: 2},
	}
	if diff := Diff(p.values, expectedValues, DiffUnexported); diff != "" {
		t.Fatalf("p.values (-got, +want):\n%s", diff)
	}

	ref6 := p.Put(likeInt(42))
	if ref6 != ref4 {
		t.Fatal("p.Put() did not reuse free slot")
	}

	expectedValues = []internValue[likeInt]{
		{},
		{value: 10, refCount: 2},
		{value: 12, refCount: 1, next: 4},
		{value: 42, refCount: 1, previous: 4},
		{value: 32, refCount: 1, previous: 2, next: 3},
	}
	if diff := Diff(p.values, expectedValues, DiffUnexported); diff != "" {
		t.Fatalf("p.values (-got, +want):\n%s", diff)
	}

	p.Take(ref3)

	expectedValues = []internValue[likeInt]{
		{},
		{value: 10, refCount: 2},
		{value: 12, refCount: 0, next: 4}, // free
		{value: 42, refCount: 1, previous: 4},
		{value: 32, refCount: 1, next: 3},
	}
	if diff := Diff(p.values, expectedValues, DiffUnexported); diff != "" {
		t.Fatalf("p.values (-got, +want):\n%s", diff)
	}

	p.Take(ref5)

	expectedValues = []internValue[likeInt]{
		{},
		{value: 10, refCount: 2},
		{value: 12, refCount: 0, next: 4}, // free
		{value: 42, refCount: 1},
		{value: 32, refCount: 0, next: 3}, // free
	}
	if diff := Diff(p.values, expectedValues, DiffUnexported); diff != "" {
		t.Fatalf("p.values (-got, +want):\n%s", diff)
	}

	p.Take(ref6)

	expectedValues = []internValue[likeInt]{
		{},
		{value: 10, refCount: 2},
		{value: 12, refCount: 0, next: 4}, // free
		{value: 42, refCount: 0},          // free
		{value: 32, refCount: 0, next: 3}, // free
	}
	if diff := Diff(p.values, expectedValues, DiffUnexported); diff != "" {
		t.Fatalf("p.values (-got, +want):\n%s", diff)
	}

	p.Take(ref1)
	p.Take(ref2)
	diff := p.Len()
	if diff != 0 {
		t.Fatalf("Take() didn't free everything (%d remaining)", diff)
	}
}
