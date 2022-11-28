// SPDX-FileCopyrightText: 2009 The Go Authors
// SPDX-License-Identifier: BSD-3-Clause

package sync

import (
	_ "unsafe" // use of go:linkname
)

//go:linkname semacquire sync.runtime_Semacquire
func semacquire(addr *uint32)

//go:linkname semacquireMutex sync.runtime_SemacquireMutex
func semacquireMutex(s *uint32, lifo bool, skipframes int)

//go:linkname semrelease sync.runtime_Semrelease
func semrelease(addr *uint32, handoff bool, skipframes int)
