// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cache_test

import (
	"net/netip"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/helpers/cache"
)

func expectCacheGet(t *testing.T, c *cache.Cache[netip.Addr, string], key string, expectedResult string, expectedOk bool) {
	t.Helper()
	ip := netip.MustParseAddr(key)
	ip = netip.AddrFrom16(ip.As16())
	result, ok := c.Get(time.Time{}, ip)
	got := struct {
		Result string
		Ok     bool
	}{result, ok}
	expected := struct {
		Result string
		Ok     bool
	}{expectedResult, expectedOk}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Get() (-got, +want):\n%s", diff)
	}
}

func TestGetPut(t *testing.T) {
	c := cache.New[netip.Addr, string]()
	t1 := time.Date(2022, time.December, 31, 10, 23, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	t3 := t2.Add(time.Minute)
	c.Put(t1, netip.MustParseAddr("::ffff:127.0.0.1"), "entry1")
	c.Put(t2, netip.MustParseAddr("::ffff:127.0.0.2"), "entry2")
	c.Put(t3, netip.MustParseAddr("::ffff:127.0.0.3"), "entry3")
	c.Put(t3, netip.MustParseAddr("::ffff:127.0.0.3"), "entry4")

	expectCacheGet(t, c, "127.0.0.1", "entry1", true)
	expectCacheGet(t, c, "127.0.0.2", "entry2", true)
	expectCacheGet(t, c, "127.0.0.3", "entry4", true)
	expectCacheGet(t, c, "127.0.0.4", "", false)

	got := c.Items()
	expected := map[netip.Addr]string{
		netip.MustParseAddr("::ffff:127.0.0.1"): "entry1",
		netip.MustParseAddr("::ffff:127.0.0.2"): "entry2",
		netip.MustParseAddr("::ffff:127.0.0.3"): "entry4",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Items() (-got, +want):\n%s", diff)
	}
}

func TestDeleteLastAccessedBefore(t *testing.T) {
	c := cache.New[netip.Addr, string]()
	t1 := time.Date(2022, time.December, 31, 10, 23, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	t3 := t2.Add(time.Minute)
	c.Put(t1, netip.MustParseAddr("::ffff:127.0.0.1"), "entry1")
	c.Put(t2, netip.MustParseAddr("::ffff:127.0.0.2"), "entry2")
	c.Put(t3, netip.MustParseAddr("::ffff:127.0.0.3"), "entry3")

	t4 := t3.Add(time.Minute)
	c.Get(t4, netip.MustParseAddr("::ffff:127.0.0.1"))
	c.Get(time.Time{}, netip.MustParseAddr("::ffff:127.0.0.2"))

	c.DeleteLastAccessedBefore(t1)
	expectCacheGet(t, c, "127.0.0.1", "entry1", true)
	expectCacheGet(t, c, "127.0.0.2", "entry2", true)
	expectCacheGet(t, c, "127.0.0.3", "entry3", true)

	c.DeleteLastAccessedBefore(t2)
	expectCacheGet(t, c, "127.0.0.1", "entry1", true)
	expectCacheGet(t, c, "127.0.0.2", "entry2", true)
	expectCacheGet(t, c, "127.0.0.3", "entry3", true)

	c.DeleteLastAccessedBefore(t3)
	expectCacheGet(t, c, "127.0.0.1", "entry1", true)
	expectCacheGet(t, c, "127.0.0.2", "", false)
	expectCacheGet(t, c, "127.0.0.3", "entry3", true)

	if count := c.DeleteLastAccessedBefore(t4); count != 1 {
		t.Errorf("DeleteLastAccessedBefore(): got %d, expected %d", count, 1)
	}
	expectCacheGet(t, c, "127.0.0.1", "entry1", true)
	expectCacheGet(t, c, "127.0.0.2", "", false)
	expectCacheGet(t, c, "127.0.0.3", "", false)

	c.DeleteLastAccessedBefore(t4.Add(time.Minute))
	expectCacheGet(t, c, "127.0.0.1", "", false)
	expectCacheGet(t, c, "127.0.0.2", "", false)
	expectCacheGet(t, c, "127.0.0.3", "", false)
}

func TestItemsLastUpdatedBefore(t *testing.T) {
	c := cache.New[netip.Addr, string]()
	t1 := time.Date(2022, time.December, 31, 10, 23, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	t3 := t2.Add(time.Minute)
	c.Put(t1, netip.MustParseAddr("::ffff:127.0.0.1"), "entry1")
	c.Put(t2, netip.MustParseAddr("::ffff:127.0.0.2"), "entry2")
	c.Put(t3, netip.MustParseAddr("::ffff:127.0.0.3"), "entry3")

	t4 := t3.Add(time.Minute)
	c.Put(t4, netip.MustParseAddr("::ffff:127.0.0.1"), "entry4")

	got := c.ItemsLastUpdatedBefore(t3)
	expected := map[netip.Addr]string{
		netip.MustParseAddr("::ffff:127.0.0.2"): "entry2",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("ItemsLastUpdatedBefore() (-got, +want):\n%s", diff)
	}
}
