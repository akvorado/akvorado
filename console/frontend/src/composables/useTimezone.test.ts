// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, beforeEach } from "vitest";
import { Date as SugarDate } from "sugar-date";
import { time, number } from "echarts/core";
import {
  selectedTimezone,
  timezones,
  selectedTimezoneItem,
} from "./useTimezone";

describe("useTimezone composable", () => {
  beforeEach(() => {
    selectedTimezone.value = "browser";
  });

  it("initializes with browser as the default timezone", () => {
    expect(selectedTimezone.value).toBe("browser");
    expect(timezones.length).toBeGreaterThan(1);
    expect(timezones[0].code).toBe("browser");
    expect(timezones[0].name).toBe("Browser time");
    expect(timezones[1].code).toBe("UTC");
  });

  it("updates reactive state when timezone changes", () => {
    selectedTimezone.value = "UTC";
    expect(selectedTimezoneItem.value.code).toBe("UTC");

    const nyItem = timezones.find((t) => t.code === "America/New_York");
    if (nyItem) {
      selectedTimezoneItem.value = nyItem;
      expect(selectedTimezone.value).toBe("America/New_York");
    }
  });

  it("patches Date getters, setters, and constructor for UTC and IANA timezones", () => {
    // 2026-09-19T22:30:45.123Z -> 2026-09-20 00:30:45.123 in Europe/Paris (CEST, UTC+2)
    const ts = Date.UTC(2026, 8, 19, 22, 30, 45, 123);
    const d = new Date(ts);

    selectedTimezone.value = "UTC";
    expect(d.getFullYear()).toBe(2026);
    expect(d.getMonth()).toBe(8);
    expect(d.getDate()).toBe(19);
    expect(d.getDay()).toBe(6); // Saturday
    expect(d.getHours()).toBe(22);
    expect(d.getMinutes()).toBe(30);
    expect(d.getSeconds()).toBe(45);
    expect(d.getMilliseconds()).toBe(123);
    expect(d.getTimezoneOffset()).toBe(0);
    expect(d.toDateString()).toBe("Sat Sep 19 2026");

    selectedTimezone.value = "Europe/Paris";
    expect(d.getFullYear()).toBe(2026);
    expect(d.getMonth()).toBe(8);
    expect(d.getDate()).toBe(20);
    expect(d.getDay()).toBe(0); // Sunday
    expect(d.getHours()).toBe(0);
    expect(d.getMinutes()).toBe(30);
    expect(d.getSeconds()).toBe(45);
    expect(d.getMilliseconds()).toBe(123);
    expect(d.getTimezoneOffset()).toBe(-120);
    expect(d.toDateString()).toBe("Sun Sep 20 2026");
    expect(d.toTimeString()).toBe("00:30:45 GMT+0200");

    // Multi-arg Date constructor in Europe/Paris
    const constructed = new Date(2026, 8, 20, 0, 30, 45, 123);
    expect(constructed.getTime()).toBe(ts);

    // Setting hours to midnight in Europe/Paris -> 2026-09-19T22:00:00.000Z
    constructed.setHours(0, 0, 0, 0);
    expect(constructed.toISOString()).toBe("2026-09-19T22:00:00.000Z");
  });

  it("handles DST transitions and Year 1 AD in setters without looping", () => {
    selectedTimezone.value = "Europe/Paris";

    // Spring-Forward in Europe/Paris: 2026-03-29 01:00 CET (00:00Z) -> +1h -> 03:00 CEST (01:00Z)
    const spring = new Date(Date.UTC(2026, 2, 29, 0, 0, 0));
    expect(spring.getHours()).toBe(1);
    spring.setHours(spring.getHours() + 1);
    expect(spring.toISOString()).toBe("2026-03-29T01:00:00.000Z");
    expect(spring.getHours()).toBe(3);

    // ECharts createEstimateNiceMultiple Year 1 AD check
    const tmp = new Date(0);
    tmp.setFullYear(1);
    const t1 = tmp.getTime();
    tmp.setFullYear(2);
    const t2 = tmp.getTime();
    expect(t2 - t1).toBeGreaterThan(360 * 86400_000);
    expect(t2 - t1).toBeLessThan(370 * 86400_000);
  });

  it("integrates with SugarDate parsing and formatting", () => {
    selectedTimezone.value = "America/New_York"; // EDT (UTC-4) in Oct 2026

    // Date-only string "2026-10-07" should be parsed as midnight in America/New_York (04:00Z)
    const parsedDateOnly = SugarDate.create("2026-10-07");
    expect(parsedDateOnly.toISOString()).toBe("2026-10-07T04:00:00.000Z");

    const parsedDateTime = SugarDate.create("2026-09-13 08:00:00");
    expect(parsedDateTime.toISOString()).toBe("2026-09-13T12:00:00.000Z");

    const formatted = SugarDate(parsedDateTime).format(
      "{yyyy}-{MM}-{dd} {HH}:{mm}:{ss}",
    ).raw;
    expect(formatted).toBe("2026-09-13 08:00:00");
  });

  it("integrates with ECharts time formatting and half-hour/European midnight alignment", () => {
    // 1. Half-hour timezone (Asia/Kolkata, UTC+05:30): rounding to midnight aligns with 18:30 UTC
    selectedTimezone.value = "Asia/Kolkata";
    const kolkataDate = new Date(Date.UTC(2026, 8, 19, 20, 15, 0));
    kolkataDate.setHours(0, 0, 0, 0);
    expect(kolkataDate.toISOString()).toBe("2026-09-19T18:30:00.000Z");
    expect(kolkataDate.getHours()).toBe(0);
    expect(kolkataDate.getMinutes()).toBe(0);
    expect(
      time.format(kolkataDate, "{yyyy}-{MM}-{dd} {HH}:{mm}:{ss}", false),
    ).toBe("2026-09-20 00:00:00");

    // 2. Europe/Paris (CEST, UTC+2): midnight is only at 22:00 UTC, and 00:00 UTC is 02:00
    selectedTimezone.value = "Europe/Paris";
    const parisMidnight = new Date(Date.UTC(2026, 8, 19, 22, 0, 0));
    expect(parisMidnight.getHours()).toBe(0);
    expect(
      time.format(parisMidnight, "{yyyy}-{MM}-{dd} {HH}:{mm}:{ss}", false),
    ).toBe("2026-09-20 00:00:00");

    const utcMidnightInParis = new Date(Date.UTC(2026, 8, 20, 0, 0, 0));
    expect(utcMidnightInParis.getHours()).toBe(2);
    expect(
      time.format(utcMidnightInParis, "{yyyy}-{MM}-{dd} {HH}:{mm}:{ss}", false),
    ).toBe("2026-09-20 02:00:00");

    // ECharts parseDate without timezone offset uses selectedTimezone
    const parsedByEcharts = number.parseDate("2026-09-20 00:00:00");
    expect(parsedByEcharts.toISOString()).toBe("2026-09-19T22:00:00.000Z");
  });
});
