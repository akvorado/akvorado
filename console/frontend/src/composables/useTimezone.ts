// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { useStorage } from "@vueuse/core";
import { computed } from "vue";

export type TimezoneItem = {
  id: number;
  code: string;
  name: string;
};

// User's browser timezone (e.g. "Europe/Paris", "America/New_York")
export const browserTimezone =
  typeof Intl !== "undefined" && typeof Intl.DateTimeFormat === "function"
    ? Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"
    : "UTC";

// Persistent storage for selected timezone (default: "browser")
export const selectedTimezone = useStorage<string>(
  "akvorado-timezone",
  "browser",
);

export const isBrowser = computed(() => selectedTimezone.value === "browser");
export const isUTC = computed(() => selectedTimezone.value === "UTC");

// Pre-computed list of timezone options
export const timezones: TimezoneItem[] = [
  { id: 1, code: "browser", name: `Browser Time (${browserTimezone})` },
  { id: 2, code: "UTC", name: "UTC" },
  ...(typeof Intl !== "undefined" &&
  typeof Intl.supportedValuesOf === "function"
    ? Intl.supportedValuesOf("timeZone")
        .filter((tz) => tz !== "UTC")
        .map((tz, idx) => ({ id: idx + 3, code: tz, name: tz }))
    : []),
];

export const selectedTimezoneItem = computed<TimezoneItem>({
  get: () =>
    timezones.find((tz) => tz.code === selectedTimezone.value) ?? timezones[0],
  set: (item) => {
    if (item && item.code) {
      selectedTimezone.value = item.code;
    }
  },
});

export const formatTimezoneTooltip = (val: string | number): string => {
  const date = new Date(val);
  if (isNaN(date.getTime())) return String(val);

  const tz = isBrowser.value ? undefined : selectedTimezone.value;
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: tz,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    timeZoneName: "short",
  })
    .format(date)
    .replace(",", "");
};

export const timezoneAxisFormatter = (val: number): string => {
  if (isBrowser.value || isUTC.value) {
    return "";
  }
  const date = new Date(val);
  const tz = selectedTimezone.value;
  const timeStr = new Intl.DateTimeFormat("en-GB", {
    timeZone: tz,
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date);

  if (timeStr === "00:00") {
    const dateStr = new Intl.DateTimeFormat("en-GB", {
      timeZone: tz,
      month: "short",
      day: "numeric",
    }).format(date);
    return `${dateStr}\n${timeStr}`;
  }
  return timeStr;
};
