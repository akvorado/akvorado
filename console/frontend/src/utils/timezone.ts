// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { useLocalStorage } from "@vueuse/core";

export interface TimezoneOption {
  id: number;
  name: string;
  code: string;
}

const buildTimezoneList = (): TimezoneOption[] => {
  const options: TimezoneOption[] = [
    { id: 1, name: "Browser time", code: "browser" },
    { id: 2, name: "UTC", code: "UTC" },
  ];

  try {
    const supported = Intl.supportedValuesOf("timeZone");
    let id = 3;
    for (const tz of supported) {
      if (tz === "UTC") continue;
      options.push({ id: id++, name: tz, code: tz });
    }
  } catch {
    // Fallback if supportedValuesOf is unavailable
  }

  return options;
};

export const timezones: TimezoneOption[] = buildTimezoneList();
export const selectedTimezone = useLocalStorage("timezone", "browser");

// Patch global Date so libraries that rely on local Date methods (ECharts,
// SugarDate) transparently operate in the user-selected timezone.
const OrigDate = globalThis.Date;
const proto = OrigDate.prototype;

function getOrigMethod<T>(key: keyof Date): T {
  return Object.getOwnPropertyDescriptor(proto, key)!.value as T;
}

const origGetTimezoneOffset =
  getOrigMethod<(this: Date) => number>("getTimezoneOffset");
const origToDateString = getOrigMethod<(this: Date) => string>("toDateString");
const origToTimeString = getOrigMethod<(this: Date) => string>("toTimeString");
const origToString = getOrigMethod<(this: Date) => string>("toString");

const DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const MONTHS = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

const dtfCache = new Map<string, Intl.DateTimeFormat>();
const offsetCache = new Map<string, number>();
const scratchDate = new OrigDate(0);
const calcDate = new OrigDate(0);

function getFormatter(tz: string): Intl.DateTimeFormat {
  let dtf = dtfCache.get(tz);
  if (!dtf) {
    dtf = new Intl.DateTimeFormat("en-US", {
      timeZone: tz,
      year: "numeric",
      month: "numeric",
      day: "numeric",
      hour: "numeric",
      minute: "numeric",
      second: "numeric",
      hour12: false,
    });
    dtfCache.set(tz, dtf);
  }
  return dtf;
}

function getOffsetMs(utcMs: number, tz: string): number {
  if (tz === "UTC") return 0;
  // IANA timezone transitions since 1970 occur on 15-minute (900_000 ms) boundaries.
  const bucket = Math.floor(utcMs / 900_000);
  const key = `${tz}:${bucket}`;
  let offset = offsetCache.get(key);
  if (offset !== undefined) return offset;

  const sampleMs = bucket * 900_000;
  try {
    const parts = getFormatter(tz).formatToParts(new OrigDate(sampleMs));
    let y = 0,
      m = 0,
      d = 0,
      h = 0,
      min = 0,
      s = 0;
    for (const p of parts) {
      switch (p.type) {
        case "year":
          y = +p.value;
          break;
        case "month":
          m = +p.value;
          break;
        case "day":
          d = +p.value;
          break;
        case "hour":
          h = +p.value % 24;
          break;
        case "minute":
          min = +p.value;
          break;
        case "second":
          s = +p.value;
          break;
      }
    }
    calcDate.setUTCFullYear(y, m - 1, d);
    calcDate.setUTCHours(h, min, s, 0);
    offset = calcDate.getTime() - sampleMs;
  } catch {
    return 0;
  }
  if (offsetCache.size > 4096) offsetCache.clear();
  offsetCache.set(key, offset);
  return offset;
}

function localToUtcMs(localMs: number, tz: string, refUtcMs?: number): number {
  if (tz === "UTC") return localMs;
  const offset0 = getOffsetMs(
    refUtcMs !== undefined && !Number.isNaN(refUtcMs) ? refUtcMs : localMs,
    tz,
  );
  const guess1 = localMs - offset0;
  const offset1 = getOffsetMs(guess1, tz);
  if (offset1 === offset0) return guess1;
  const guess2 = localMs - offset1;
  const offset2 = getOffsetMs(guess2, tz);
  if (offset2 === offset1) return guess2;
  return localMs - Math.min(offset1, offset2);
}

type UTCGetterKey =
  | "getUTCFullYear"
  | "getUTCMonth"
  | "getUTCDate"
  | "getUTCDay"
  | "getUTCHours"
  | "getUTCMinutes"
  | "getUTCSeconds"
  | "getUTCMilliseconds";

function makeGetter(localKey: keyof Date, utcKey: UTCGetterKey) {
  const origLocal = getOrigMethod<(this: Date) => number>(localKey);
  return function (this: Date): number {
    const tz = selectedTimezone.value;
    if (tz === "browser") return origLocal.call(this);
    if (tz === "UTC") return this[utcKey]();
    const ms = this.getTime();
    if (Number.isNaN(ms)) return Number.NaN;
    scratchDate.setTime(ms + getOffsetMs(ms, tz));
    return scratchDate[utcKey]();
  };
}

function makeSetter(
  localKey: keyof Date,
  utcKey: keyof Date,
  nanFallbackZero = false,
) {
  const origLocal =
    getOrigMethod<(this: Date, ...args: number[]) => number>(localKey);
  const origUTC =
    getOrigMethod<(this: Date, ...args: number[]) => number>(utcKey);
  return function (this: Date, ...args: number[]): number {
    const tz = selectedTimezone.value;
    if (tz === "browser") return origLocal.apply(this, args);
    if (tz === "UTC") return origUTC.apply(this, args);
    const ms = this.getTime();
    if (Number.isNaN(ms)) {
      if (!nanFallbackZero) return Number.NaN;
      scratchDate.setTime(0);
    } else {
      scratchDate.setTime(ms + getOffsetMs(ms, tz));
    }
    const newLocalMs = origUTC.apply(scratchDate, args);
    if (Number.isNaN(newLocalMs)) {
      return this.setTime(Number.NaN);
    }
    return this.setTime(localToUtcMs(newLocalMs, tz, ms));
  };
}

proto.getFullYear = makeGetter("getFullYear", "getUTCFullYear");
proto.getMonth = makeGetter("getMonth", "getUTCMonth");
proto.getDate = makeGetter("getDate", "getUTCDate");
proto.getDay = makeGetter("getDay", "getUTCDay");
proto.getHours = makeGetter("getHours", "getUTCHours");
proto.getMinutes = makeGetter("getMinutes", "getUTCMinutes");
proto.getSeconds = makeGetter("getSeconds", "getUTCSeconds");
proto.getMilliseconds = makeGetter("getMilliseconds", "getUTCMilliseconds");

proto.getTimezoneOffset = function (this: Date): number {
  const tz = selectedTimezone.value;
  if (tz === "browser") return origGetTimezoneOffset.call(this);
  const ms = this.getTime();
  if (Number.isNaN(ms)) return Number.NaN;
  const offsetMin = -getOffsetMs(ms, tz) / 60_000;
  return offsetMin === 0 ? 0 : offsetMin;
};

proto.setFullYear = makeSetter("setFullYear", "setUTCFullYear", true);
proto.setMonth = makeSetter("setMonth", "setUTCMonth");
proto.setDate = makeSetter("setDate", "setUTCDate");
proto.setHours = makeSetter("setHours", "setUTCHours");
proto.setMinutes = makeSetter("setMinutes", "setUTCMinutes");
proto.setSeconds = makeSetter("setSeconds", "setUTCSeconds");
proto.setMilliseconds = makeSetter("setMilliseconds", "setUTCMilliseconds");

proto.toDateString = function (this: Date): string {
  const tz = selectedTimezone.value;
  if (tz === "browser") return origToDateString.call(this);
  const ms = this.getTime();
  if (Number.isNaN(ms)) return "Invalid Date";
  scratchDate.setTime(ms + getOffsetMs(ms, tz));
  const dayName = DAYS[scratchDate.getUTCDay()];
  const monthName = MONTHS[scratchDate.getUTCMonth()];
  const dateStr = String(scratchDate.getUTCDate()).padStart(2, "0");
  const yearStr = String(scratchDate.getUTCFullYear()).padStart(4, "0");
  return `${dayName} ${monthName} ${dateStr} ${yearStr}`;
};

proto.toTimeString = function (this: Date): string {
  const tz = selectedTimezone.value;
  if (tz === "browser") return origToTimeString.call(this);
  const ms = this.getTime();
  if (Number.isNaN(ms)) return "Invalid Date";
  const offsetMs = getOffsetMs(ms, tz);
  scratchDate.setTime(ms + offsetMs);
  const hh = String(scratchDate.getUTCHours()).padStart(2, "0");
  const mm = String(scratchDate.getUTCMinutes()).padStart(2, "0");
  const ss = String(scratchDate.getUTCSeconds()).padStart(2, "0");
  const offsetMin = Math.round(offsetMs / 60_000);
  const sign = offsetMin >= 0 ? "+" : "-";
  const absMin = Math.abs(offsetMin);
  const offH = String(Math.floor(absMin / 60)).padStart(2, "0");
  const offM = String(absMin % 60).padStart(2, "0");
  return `${hh}:${mm}:${ss} GMT${sign}${offH}${offM}`;
};

proto.toString = function (this: Date): string {
  const tz = selectedTimezone.value;
  if (tz === "browser") return origToString.call(this);
  const ms = this.getTime();
  if (Number.isNaN(ms)) return "Invalid Date";
  return `${this.toDateString()} ${this.toTimeString()}`;
};

globalThis.Date = new Proxy(OrigDate, {
  construct(target, args: unknown[], newTarget) {
    const tz = selectedTimezone.value;
    if (tz !== "browser" && args.length >= 2) {
      const [y, m, ...rest] = args as [number, number, ...number[]];
      const localMs = OrigDate.UTC(y, m, ...rest);
      const utcMs = Number.isNaN(localMs)
        ? Number.NaN
        : localToUtcMs(localMs, tz);
      return Reflect.construct(target, [utcMs], newTarget);
    }
    return Reflect.construct(target, args, newTarget);
  },
  apply() {
    return new OrigDate().toString();
  },
});
