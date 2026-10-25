// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build akvorado_inlet

package httpserver

// CacheBackendConfiguration represents the configuration of a cache backend.
// The cache is not available in this build.
type CacheBackendConfiguration any

type cache struct{}

func (c *Component) startCache() error { return nil }

func (c *Component) stopCache() {}
