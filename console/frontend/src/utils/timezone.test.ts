// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from "vitest";
import {
  formatOffsetString,
  getTimezoneOffsetMinutes,
  shiftDateForEcharts,
  unshiftDateFromEcharts,
  formatDateInTimezone,
  searchTimezones,
  getQuickTimezones,
  getAllTimezoneItems,
} from "./timezone";

describe("formatOffsetString", () => {
  it("formats positive offsets correctly", () => {
    expect(formatOffsetString(0)).toBe("UTC+00:00");
    expect(formatOffsetString(60)).toBe("UTC+01:00");
    expect(formatOffsetString(120)).toBe("UTC+02:00");
    expect(formatOffsetString(330)).toBe("UTC+05:30");
  });

  it("formats negative offsets correctly", () => {
    expect(formatOffsetString(-240)).toBe("UTC-04:00");
    expect(formatOffsetString(-300)).toBe("UTC-05:00");
    expect(formatOffsetString(-210)).toBe("UTC-03:30");
  });
});

describe("getTimezoneOffsetMinutes", () => {
  it("returns 0 for UTC", () => {
    expect(getTimezoneOffsetMinutes("UTC")).toBe(0);
    expect(getTimezoneOffsetMinutes("Etc/UTC")).toBe(0);
  });

  it("calculates correct offset for specific dates in standard timezones", () => {
    // September (summer in northern hemisphere)
    const sepDate = new Date("2026-09-13T12:00:00Z");
    // Europe/Paris is CEST (UTC+2 = 120 mins)
    expect(getTimezoneOffsetMinutes("Europe/Paris", sepDate)).toBe(120);
    // America/New_York is EDT (UTC-4 = -240 mins)
    expect(getTimezoneOffsetMinutes("America/New_York", sepDate)).toBe(-240);
    // Asia/Tokyo is JST (UTC+9 = 540 mins)
    expect(getTimezoneOffsetMinutes("Asia/Tokyo", sepDate)).toBe(540);
  });
});

describe("shiftDateForEcharts and unshiftDateFromEcharts", () => {
  it("leaves browser and UTC dates untouched", () => {
    const d = new Date("2026-09-13T12:00:00Z");
    expect(shiftDateForEcharts(d, "UTC").getTime()).toBe(d.getTime());
    expect(unshiftDateFromEcharts(d, "UTC").getTime()).toBe(d.getTime());
    expect(shiftDateForEcharts(d, "browser").getTime()).toBe(d.getTime());
    expect(unshiftDateFromEcharts(d, "browser").getTime()).toBe(d.getTime());
  });

  it("shifts dates by timezone offset so UTC represents target wall-clock time", () => {
    const original = new Date("2026-09-13T12:00:00Z");
    const shifted = shiftDateForEcharts(original, "America/New_York");
    // NY is UTC-4 in Sep, so 12:00 UTC is 08:00 NY time
    expect(shifted.toISOString()).toBe("2026-09-13T08:00:00.000Z");

    // Unshifting restores original UTC date
    const unshifted = unshiftDateFromEcharts(shifted, "America/New_York");
    expect(unshifted.getTime()).toBe(original.getTime());
  });

  it("handles Paris time shift and unshift accurately", () => {
    const original = new Date("2026-09-13T12:00:00Z");
    const shifted = shiftDateForEcharts(original, "Europe/Paris");
    // Paris is UTC+2 in Sep, so 12:00 UTC is 14:00 Paris time
    expect(shifted.toISOString()).toBe("2026-09-13T14:00:00.000Z");

    const unshifted = unshiftDateFromEcharts(shifted, "Europe/Paris");
    expect(unshifted.getTime()).toBe(original.getTime());
  });
});

describe("formatDateInTimezone", () => {
  const d = new Date("2026-09-13T12:30:45Z");

  it("formats in UTC correctly", () => {
    expect(formatDateInTimezone(d, "UTC", "timeOnly")).toBe("12:30:45");
    expect(formatDateInTimezone(d, "UTC", "iso")).toBe("2026-09-13 12:30:45");
  });

  it("formats in America/New_York correctly", () => {
    expect(formatDateInTimezone(d, "America/New_York", "timeOnly")).toBe(
      "08:30:45",
    );
    expect(formatDateInTimezone(d, "America/New_York", "iso")).toBe(
      "2026-09-13 08:30:45",
    );
  });

  it("formats in Europe/Paris correctly", () => {
    expect(formatDateInTimezone(d, "Europe/Paris", "timeOnly")).toBe(
      "14:30:45",
    );
    expect(formatDateInTimezone(d, "Europe/Paris", "iso")).toBe(
      "2026-09-13 14:30:45",
    );
  });
});

describe("searchTimezones", () => {
  it("returns quick options and grouped items on empty query", () => {
    const res = searchTimezones("");
    expect(res.quick.length).toBeGreaterThanOrEqual(2);
    expect(res.quick.some((item) => item.id === "browser")).toBe(true);
    expect(res.quick.some((item) => item.id === "UTC")).toBe(true);
    expect(res.groups.length).toBeGreaterThan(0);
  });

  it("filters timezones by city or name", () => {
    const res = searchTimezones("Paris");
    const found = res.groups
      .flatMap((g) => g.items)
      .some((item) => item.id === "Europe/Paris");
    expect(found).toBe(true);
  });

  it("filters quick options when query matches UTC", () => {
    const res = searchTimezones("UTC");
    expect(res.quick.some((item) => item.id === "UTC")).toBe(true);
  });
});

describe("getQuickTimezones and getAllTimezoneItems", () => {
  it("returns quick timezone items including browser and UTC", () => {
    const quick = getQuickTimezones();
    expect(quick.length).toBe(2);
    expect(quick[0].id).toBe("browser");
    expect(quick[1].id).toBe("UTC");
  });

  it("returns a non-empty list of all timezone items", () => {
    const all = getAllTimezoneItems();
    expect(all.length).toBeGreaterThan(10);
    expect(all.some((tz) => tz.id === "Europe/Paris")).toBe(true);
  });
});
