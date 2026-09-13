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
});
