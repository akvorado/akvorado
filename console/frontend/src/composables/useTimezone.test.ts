// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, beforeEach } from "vitest";
import {
  selectedTimezone,
  isBrowser,
  isUTC,
  timezones,
  selectedTimezoneItem,
  formatTimezoneTooltip,
  formatTimezoneDateTime,
  parseTimezoneDate,
  convertTimezoneString,
  formatTimezoneAxisPointer,
  timezoneAxisFormatter,
  formatTimezoneSummary,
} from "./useTimezone";

describe("useTimezone composable", () => {
  beforeEach(() => {
    selectedTimezone.value = "browser";
  });

  it("provides initial browser timezone settings", () => {
    expect(selectedTimezone.value).toBe("browser");
    expect(isBrowser.value).toBe(true);
    expect(isUTC.value).toBe(false);
    expect(timezones.length).toBeGreaterThan(1);
    expect(timezones[0].code).toBe("browser");
    expect(timezones[0].name).toBe("Browser time");
    expect(timezones[1].code).toBe("UTC");
  });

  it("updates timezone selection and flags", () => {
    selectedTimezone.value = "UTC";
    expect(isBrowser.value).toBe(false);
    expect(isUTC.value).toBe(true);
    expect(selectedTimezoneItem.value.code).toBe("UTC");

    selectedTimezoneItem.value = {
      id: 999,
      code: "Asia/Tokyo",
      name: "Asia/Tokyo",
    };
    expect(selectedTimezone.value).toBe("Asia/Tokyo");
    expect(isBrowser.value).toBe(false);
    expect(isUTC.value).toBe(false);
  });

  it("formats tooltip timestamps correctly for UTC", () => {
    selectedTimezone.value = "UTC";
    // 2026-09-13T12:00:00Z
    const ts = Date.UTC(2026, 8, 13, 12, 0, 0);
    const formatted = formatTimezoneTooltip(ts);
    expect(formatted).toContain("2026-09-13");
    expect(formatted).toContain("12:00:00");
    expect(formatted).toContain("UTC");
  });

  it("formats tooltip timestamps correctly for a custom timezone", () => {
    selectedTimezone.value = "America/New_York";
    // 2026-09-13T12:00:00Z is 08:00:00 EDT
    const ts = Date.UTC(2026, 8, 13, 12, 0, 0);
    const formatted = formatTimezoneTooltip(ts);
    expect(formatted).toContain("2026-09-13");
    expect(formatted).toContain("08:00:00");
  });

  it("formats and parses timezone date strings accurately", () => {
    const ts = Date.UTC(2026, 8, 13, 14, 30, 0);
    expect(formatTimezoneDateTime(ts, "UTC")).toBe("2026-09-13 14:30:00");
    expect(formatTimezoneDateTime(ts, "America/New_York")).toBe(
      "2026-09-13 10:30:00",
    );

    const parsedUtc = parseTimezoneDate("2026-09-13 14:30:00", "UTC");
    expect(parsedUtc.toISOString()).toBe("2026-09-13T14:30:00.000Z");

    const parsedNy = parseTimezoneDate(
      "2026-09-13 10:30:00",
      "America/New_York",
    );
    expect(parsedNy.toISOString()).toBe("2026-09-13T14:30:00.000Z");
  });

  it("converts timezone strings between timezones", () => {
    const converted = convertTimezoneString(
      "2026-09-13 10:30:00",
      "America/New_York",
      "UTC",
    );
    expect(converted).toBe("2026-09-13 14:30:00");

    const convertedIso = convertTimezoneString(
      "2026-09-13T14:30:00.000Z",
      "UTC",
      "America/New_York",
    );
    expect(convertedIso).toBe("2026-09-13 10:30:00");

    expect(convertTimezoneString("1 hour ago", "UTC", "America/New_York")).toBe(
      "1 hour ago",
    );
  });

  it("formats axis pointer correctly", () => {
    selectedTimezone.value = "UTC";
    const ts = Date.UTC(2026, 8, 13, 14, 30, 0);
    expect(formatTimezoneAxisPointer(ts)).toBe("2026-09-13 14:30:00");
  });

  it("formats axis labels with multi-level awareness and midnight day conversion", () => {
    selectedTimezone.value = "Europe/Paris"; // UTC+2 in Sept

    // Regular hour tick (21:00 UTC = 23:00 Paris)
    const tick23 = Date.UTC(2026, 8, 13, 21, 0, 0);
    const label23 = timezoneAxisFormatter(tick23, 0, {
      time: { lowerTimeUnit: "hour", upperTimeUnit: "day", level: 0 },
    });
    expect(label23).toBe("23:00");

    // Midnight tick in Paris (22:00 UTC = 00:00 Paris) -> converted to day
    const tickMidnight = Date.UTC(2026, 8, 13, 22, 0, 0);
    const labelMidnight = timezoneAxisFormatter(tickMidnight, 1, {
      time: { lowerTimeUnit: "hour", upperTimeUnit: "day", level: 0 },
    });
    expect(labelMidnight).toMatch(/14/);

    // Multi-day zoomed out ticks (lowerTimeUnit === "day")
    const dayTick = Date.UTC(2026, 8, 15, 0, 0, 0);
    const dayLabel = timezoneAxisFormatter(dayTick, 0, {
      time: { lowerTimeUnit: "day", upperTimeUnit: "month", level: 0 },
    });
    expect(dayLabel).toBe("15");

    // Month boundary tick (level > 0)
    const monthTick = Date.UTC(2026, 9, 1, 0, 0, 0);
    const monthLabel = timezoneAxisFormatter(monthTick, 0, {
      time: { lowerTimeUnit: "day", upperTimeUnit: "month", level: 1 },
    });
    expect(monthLabel).toMatch(/Oct/);
  });

  it("formats summary dates with relative day awareness and 24H format", () => {
    selectedTimezone.value = "UTC";
    const start = "2026-09-13T17:45:54.000Z";
    const sameDayEnd = "2026-09-13T18:45:54.000Z";
    const nextDayEnd = "2026-09-14T18:45:54.000Z";

    const formattedStart = formatTimezoneSummary(start);
    expect(formattedStart).toContain("13");
    expect(formattedStart).toContain("17:45:54");
    expect(formattedStart).not.toMatch(/am|pm/i);

    const formattedSameDay = formatTimezoneSummary(sameDayEnd, start);
    expect(formattedSameDay).toBe("18:45:54");

    const formattedNextDay = formatTimezoneSummary(nextDayEnd, start);
    expect(formattedNextDay).toContain("14");
    expect(formattedNextDay).toContain("18:45:54");
    expect(formattedNextDay).not.toMatch(/am|pm/i);
  });
});
