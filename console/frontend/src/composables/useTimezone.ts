// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { useStorage } from "@vueuse/core";
import { computed } from "vue";
import { Date as SugarDate } from "sugar-date";

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
  { id: 1, code: "browser", name: "Browser time" },
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

export const formatTimezoneDateTime = (
  val: string | number | Date,
  timeZone?: string,
): string => {
  const d = new Date(val);
  if (isNaN(d.getTime())) return String(val);

  const tz =
    timeZone !== undefined
      ? timeZone === "browser"
        ? undefined
        : timeZone
      : isBrowser.value
        ? undefined
        : selectedTimezone.value;

  const parts = new Intl.DateTimeFormat("en-GB", {
    timeZone: tz,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).formatToParts(d);

  const map: Record<string, string> = {};
  for (const p of parts) map[p.type] = p.value;
  return `${map.year}-${map.month}-${map.day} ${map.hour}:${map.minute}:${map.second}`;
};

export const parseTimezoneDate = (str: string, timeZone?: string): Date => {
  if (!str) return new Date();
  const trimmed = str.trim();

  // If already ends with Z or has timezone offset (+/-HH:MM), native Date handles it
  if (/[zZ]|[+-]\d{2}(:?\d{2})?$/.test(trimmed)) {
    const d = new Date(trimmed);
    if (!isNaN(d.getTime())) return d;
  }

  const m = trimmed.match(
    /^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2})(?::(\d{2}))?)?$/,
  );
  if (m) {
    const [, y, mo, d, h = "00", mi = "00", s = "00"] = m;
    const tz =
      timeZone !== undefined
        ? timeZone
        : isBrowser.value
          ? "browser"
          : selectedTimezone.value;

    if (tz === "browser") {
      return new Date(+y, +mo - 1, +d, +h, +mi, +s);
    }
    if (tz === "UTC") {
      return new Date(Date.UTC(+y, +mo - 1, +d, +h, +mi, +s));
    }

    const utcGuess = Date.UTC(+y, +mo - 1, +d, +h, +mi, +s);
    const dtf = new Intl.DateTimeFormat("en-US", {
      timeZone: tz,
      year: "numeric",
      month: "numeric",
      day: "numeric",
      hour: "numeric",
      minute: "numeric",
      second: "numeric",
      hour12: false,
    });
    const parts = dtf.formatToParts(new Date(utcGuess));
    const p: Record<string, string> = {};
    for (const part of parts) p[part.type] = part.value;
    const tzAsUtc = Date.UTC(
      +p.year,
      +p.month - 1,
      +p.day,
      +p.hour === 24 ? 0 : +p.hour,
      +p.minute,
      +p.second,
    );
    const offset = tzAsUtc - utcGuess;
    return new Date(utcGuess - offset);
  }

  return SugarDate.create(trimmed);
};

export const convertTimezoneString = (
  str: string,
  oldTz: string,
  newTz: string,
): string => {
  if (!str) return str;
  const trimmed = str.trim();
  if (
    /^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2})(?::(\d{2}))?)?$/.test(
      trimmed,
    ) ||
    /[zZ]|[+-]\d{2}(:?\d{2})?$/.test(trimmed)
  ) {
    const parsedUtc = parseTimezoneDate(trimmed, oldTz);
    if (!isNaN(parsedUtc.getTime())) {
      return formatTimezoneDateTime(parsedUtc, newTz);
    }
  }
  return str; // Keep relative strings like "1 hour ago", "now"
};

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

export const formatTimezoneAxisPointer = (val: string | number): string => {
  return formatTimezoneDateTime(val);
};

export const timezoneAxisFormatter = (
  val: number,
  _idx?: number,
  extra?: {
    time?: {
      lowerTimeUnit?: string;
      upperTimeUnit?: string;
      level?: number;
    };
  },
): string => {
  if (isBrowser.value || isUTC.value) {
    return "";
  }
  const date = new Date(val);
  const tz = selectedTimezone.value;
  const time = extra?.time;
  if (!time) {
    return new Intl.DateTimeFormat("en-GB", {
      timeZone: tz,
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(date);
  }

  const { lowerTimeUnit, upperTimeUnit, level = 0 } = time;

  if (lowerTimeUnit === "millisecond" || lowerTimeUnit === "second") {
    return new Intl.DateTimeFormat("en-GB", {
      timeZone: tz,
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).format(date);
  }

  if (lowerTimeUnit === "minute") {
    const timeParts = new Intl.DateTimeFormat("en-GB", {
      timeZone: tz,
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).formatToParts(date);
    const hour = timeParts.find((p) => p.type === "hour")?.value;
    const minute = timeParts.find((p) => p.type === "minute")?.value;
    if (
      (hour === "00" && minute === "00") ||
      (level > 0 && upperTimeUnit !== "minute")
    ) {
      return new Intl.DateTimeFormat("en-GB", {
        timeZone: tz,
        month: "short",
        day: "numeric",
      }).format(date);
    }
    return `${hour}:${minute}`;
  }

  if (lowerTimeUnit === "hour") {
    const timeParts = new Intl.DateTimeFormat("en-GB", {
      timeZone: tz,
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).formatToParts(date);
    const hour = timeParts.find((p) => p.type === "hour")?.value;
    const minute = timeParts.find((p) => p.type === "minute")?.value;
    if (
      (hour === "00" && minute === "00") ||
      (level > 0 && upperTimeUnit !== "hour")
    ) {
      return new Intl.DateTimeFormat("en-GB", {
        timeZone: tz,
        month: "short",
        day: "numeric",
      }).format(date);
    }
    return `${hour}:${minute}`;
  }

  if (lowerTimeUnit === "day") {
    if (level > 0 && upperTimeUnit !== "day") {
      return new Intl.DateTimeFormat("en-GB", {
        timeZone: tz,
        month: "short",
      }).format(date);
    }
    return new Intl.DateTimeFormat("en-GB", {
      timeZone: tz,
      day: "numeric",
    }).format(date);
  }

  if (lowerTimeUnit === "month") {
    if (level > 0 && upperTimeUnit !== "month") {
      return new Intl.DateTimeFormat("en-GB", {
        timeZone: tz,
        year: "numeric",
      }).format(date);
    }
    return new Intl.DateTimeFormat("en-GB", {
      timeZone: tz,
      month: "short",
    }).format(date);
  }

  return new Intl.DateTimeFormat("en-GB", {
    timeZone: tz,
    year: "numeric",
  }).format(date);
};

export const formatTimezoneSummary = (val: string, refVal?: string): string => {
  const d = new Date(val);
  if (isNaN(d.getTime())) return val;

  const tz = isBrowser.value ? undefined : selectedTimezone.value;

  if (refVal) {
    const refDate = new Date(refVal);
    if (!isNaN(refDate.getTime())) {
      const dDay = new Intl.DateTimeFormat("en-GB", {
        timeZone: tz,
        year: "numeric",
        month: "numeric",
        day: "numeric",
      }).format(d);
      const refDay = new Intl.DateTimeFormat("en-GB", {
        timeZone: tz,
        year: "numeric",
        month: "numeric",
        day: "numeric",
      }).format(refDate);

      if (dDay === refDay) {
        return new Intl.DateTimeFormat(undefined, {
          timeZone: tz,
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
          hour12: false,
        }).format(d);
      }
    }
  }

  return new Intl.DateTimeFormat(undefined, {
    timeZone: tz,
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(d);
};
