// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package networks

import (
	"math"
	"reflect"
	"runtime"
)

// measureMemory returns an estimation of the memory taken by the published tree
// and by the pool holding the attributes. Neither can be walked to add up their
// size, so this uses the heap profile of the runtime instead: what is still
// alive and was allocated while rebuilding is the tree and its pool.
//
// The heap profile is only refreshed by a garbage collection and this does not
// force one, therefore the result lags by up to two cycles. Right after a
// rebuild, it also counts the intermediate results if a cycle happened while
// they were still alive.
//
// It relies on the sampling of the allocations done by the runtime, so this is
// a rough estimation only: around 10% off for a tree of a hundred megabytes and
// much worse below that, as the tree is mostly made of small allocations and
// each sampled one stands for many others. Use it to follow a trend, not as an
// exact figure. It returns 0 when the sampling of the allocations is disabled.
func measureMemory() int64 {
	rate := int64(runtime.MemProfileRate)
	if rate == 0 {
		return 0
	}

	// The name of the function allocating the tree and its pool. It is looked
	// up instead of being spelled out, to survive a rename.
	rebuildFunction := runtime.FuncForPC(
		reflect.ValueOf((*Component).rebuild).Pointer()).Name()

	var records []runtime.MemProfileRecord
	count, _ := runtime.MemProfile(nil, false)
	for {
		// More records can show up between the two calls, ask for some room to
		// spare and retry until everything fits.
		records = make([]runtime.MemProfileRecord, count+32)
		var ok bool
		if count, ok = runtime.MemProfile(records, false); ok {
			records = records[:count]
			break
		}
	}

	var total int64
	for _, record := range records {
		frames := runtime.CallersFrames(record.Stack())
		for {
			frame, more := frames.Next()
			if frame.Function == rebuildFunction {
				total += unsample(record.InUseObjects(), record.InUseBytes(), rate)
				break
			}
			if !more {
				break
			}
		}
	}
	return total
}

// unsample scales the bytes of the sampled allocations back to the bytes they
// stand for. This is the same computation as the one runtime/pprof does when
// writing a heap profile, but it does not export it.
func unsample(count, size, rate int64) int64 {
	if count == 0 || size == 0 {
		return 0
	}
	if rate <= 1 || count*rate <= size {
		return size
	}
	average := float64(size) / float64(count)
	return int64(float64(size) / (1 - math.Exp(-average/float64(rate))))
}
