// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

export interface TimezoneItem {
  id: string; // e.g. "browser", "UTC", "Europe/Paris", "America/New_York"
  name: string; // e.g. "Browser Time", "Coordinated Universal Time", "Paris"
  fullName: string; // e.g. "Europe/Paris" or "Browser Time"
  region: string; // e.g. "Default", "Europe", "America"
  offsetStr: string; // e.g. "UTC+02:00", "UTC+00:00"
  offsetMinutes: number; // e.g. 120, 0
  abbr: string; // e.g. "CEST", "UTC", "EDT"
  searchKey: string; // normalized string for search
}

/**
 * Returns the browser's current IANA time zone identifier.
 */
export function getBrowserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

/**
 * Formats offset in minutes to string like "UTC+03:00" or "UTC-04:00" or "UTC+00:00".
 */
export function formatOffsetString(offsetMinutes: number): string {
  const sign = offsetMinutes >= 0 ? "+" : "-";
  const absMinutes = Math.abs(offsetMinutes);
  const hours = Math.floor(absMinutes / 60);
  const minutes = absMinutes % 60;
  return `UTC${sign}${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}`;
}

/**
 * Calculates the offset in minutes for a given timezone at a specific date.
 */
export function getTimezoneOffsetMinutes(
  timeZone: string,
  date: Date = new Date(),
): number {
  if (timeZone === "browser") {
    return -date.getTimezoneOffset();
  }
  if (timeZone === "UTC" || timeZone === "Etc/UTC" || timeZone === "GMT") {
    return 0;
  }
  try {
    const utcStr = date.toLocaleString("en-US", { timeZone: "UTC" });
    const tzStr = date.toLocaleString("en-US", { timeZone });
    return (new Date(tzStr).getTime() - new Date(utcStr).getTime()) / 60000;
  } catch {
    return 0;
  }
}

/**
 * Retrieves the short abbreviation for a timezone at a specific date.
 */
export function getTimezoneAbbr(
  timeZone: string,
  date: Date = new Date(),
): string {
  if (timeZone === "UTC" || timeZone === "Etc/UTC" || timeZone === "GMT") {
    return "UTC";
  }
  const resolved = timeZone === "browser" ? undefined : timeZone;
  try {
    const parts = new Intl.DateTimeFormat("en-US", {
      timeZone: resolved,
      timeZoneName: "short",
    }).formatToParts(date);
    const tzPart = parts.find((p) => p.type === "timeZoneName");
    return tzPart ? tzPart.value : "";
  } catch {
    return "";
  }
}

/**
 * Shifts date for ECharts time scale when viewing in an arbitrary timezone.
 * For "browser" and "UTC", no shifting is needed because ECharts supports them natively.
 * For other timezones, shifts the timestamp so UTC wall clock equals local wall clock.
 */
export function shiftDateForEcharts(
  date: Date | string | number,
  timeZone: string,
): Date {
  const d = date instanceof Date ? date : new Date(date);
  if (isNaN(d.getTime())) return d;
  if (timeZone === "browser" || timeZone === "UTC") {
    return d;
  }
  const offset = getTimezoneOffsetMinutes(timeZone, d);
  return new Date(d.getTime() + offset * 60000);
}

/**
 * Inverts the shiftDateForEcharts operation (e.g. from ECharts brush selection back to UTC).
 */
export function unshiftDateFromEcharts(
  date: Date | string | number,
  timeZone: string,
): Date {
  const d = date instanceof Date ? date : new Date(date);
  if (isNaN(d.getTime())) return d;
  if (timeZone === "browser" || timeZone === "UTC") {
    return d;
  }
  const offset = getTimezoneOffsetMinutes(timeZone, d);
  const candidate = new Date(d.getTime() - offset * 60000);
  const candidateOffset = getTimezoneOffsetMinutes(timeZone, candidate);
  return new Date(d.getTime() - candidateOffset * 60000);
}

/**
 * Formats date into a string in the specified timezone.
 */
export function formatDateInTimezone(
  date: Date | string | number,
  timeZone: string,
  style: "full" | "short" | "timeOnly" | "iso" = "full",
): string {
  const d = date instanceof Date ? date : new Date(date);
  if (isNaN(d.getTime())) return "";

  const resolved = timeZone === "browser" ? undefined : timeZone;

  if (style === "timeOnly") {
    return new Intl.DateTimeFormat("en-US", {
      timeZone: resolved,
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).format(d);
  }

  if (style === "short") {
    return new Intl.DateTimeFormat("en-US", {
      timeZone: resolved,
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(d);
  }

  if (style === "iso") {
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone: resolved,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).formatToParts(d);
    const get = (type: string) => {
      const p = parts.find((part) => part.type === type);
      return p ? p.value : "";
    };
    return `${get("year")}-${get("month")}-${get("day")} ${get("hour")}:${get("minute")}:${get("second")}`;
  }

  // "full"
  return new Intl.DateTimeFormat("en-US", {
    timeZone: resolved,
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(d);
}

// Comprehensive fallback list if Intl.supportedValuesOf is unavailable
const fallbackTimezones = [
  "Africa/Cairo",
  "Africa/Casablanca",
  "Africa/Johannesburg",
  "Africa/Lagos",
  "Africa/Nairobi",
  "America/Anchorage",
  "America/Argentina/Buenos_Aires",
  "America/Bogota",
  "America/Chicago",
  "America/Denver",
  "America/Halifax",
  "America/Lima",
  "America/Los_Angeles",
  "America/Mexico_City",
  "America/New_York",
  "America/Phoenix",
  "America/Santiago",
  "America/Sao_Paulo",
  "America/Toronto",
  "America/Vancouver",
  "Asia/Bangkok",
  "Asia/Dubai",
  "Asia/Hong_Kong",
  "Asia/Jakarta",
  "Asia/Jerusalem",
  "Asia/Kolkata",
  "Asia/Manila",
  "Asia/Riyadh",
  "Asia/Seoul",
  "Asia/Shanghai",
  "Asia/Singapore",
  "Asia/Taipei",
  "Asia/Tokyo",
  "Atlantic/Azores",
  "Atlantic/Cape_Verde",
  "Atlantic/Reykjavik",
  "Australia/Adelaide",
  "Australia/Brisbane",
  "Australia/Darwin",
  "Australia/Melbourne",
  "Australia/Perth",
  "Australia/Sydney",
  "Europe/Amsterdam",
  "Europe/Athens",
  "Europe/Berlin",
  "Europe/Brussels",
  "Europe/Bucharest",
  "Europe/Budapest",
  "Europe/Copenhagen",
  "Europe/Dublin",
  "Europe/Helsinki",
  "Europe/Istanbul",
  "Europe/Kyiv",
  "Europe/Lisbon",
  "Europe/London",
  "Europe/Madrid",
  "Europe/Moscow",
  "Europe/Oslo",
  "Europe/Paris",
  "Europe/Prague",
  "Europe/Rome",
  "Europe/Stockholm",
  "Europe/Vienna",
  "Europe/Warsaw",
  "Europe/Zurich",
  "Pacific/Auckland",
  "Pacific/Fiji",
  "Pacific/Guam",
  "Pacific/Honolulu",
];

/**
 * Returns all available IANA timezones.
 */
function getAvailableTimezoneIds(): string[] {
  try {
    if (typeof Intl !== "undefined" && "supportedValuesOf" in Intl) {
      return (Intl as unknown as { supportedValuesOf: (key: string) => string[] })
        .supportedValuesOf("timeZone");
    }
  } catch {
    // fallback
  }
  return fallbackTimezones;
}

let cachedTimezoneItems: TimezoneItem[] | null = null;

/**
 * Builds and caches all TimezoneItems with their current offsets and abbreviations.
 */
export function getAllTimezoneItems(): TimezoneItem[] {
  if (cachedTimezoneItems) {
    return cachedTimezoneItems;
  }

  const now = new Date();
  const rawZones = getAvailableTimezoneIds();
  const items: TimezoneItem[] = [];

  for (const tz of rawZones) {
    const slashIdx = tz.indexOf("/");
    if (slashIdx === -1) continue; // Skip single names like UTC / GMT since they are in quick picks

    const region = tz.substring(0, slashIdx).replace(/_/g, " ");
    const name = tz.substring(slashIdx + 1).replace(/_/g, " ");
    const offsetMinutes = getTimezoneOffsetMinutes(tz, now);
    const offsetStr = formatOffsetString(offsetMinutes);
    const abbr = getTimezoneAbbr(tz, now);

    items.push({
      id: tz,
      name,
      fullName: tz,
      region,
      offsetStr,
      offsetMinutes,
      abbr,
      searchKey: `${tz} ${name} ${region} ${offsetStr.replace("UTC", "")} ${abbr}`.toLowerCase(),
    });
  }

  // Sort alphabetically by region then name
  items.sort((a, b) => {
    if (a.region !== b.region) return a.region.localeCompare(b.region);
    return a.name.localeCompare(b.name);
  });

  cachedTimezoneItems = items;
  return items;
}

/**
 * Returns quick / default options for the top of the timezone selector.
 */
export function getQuickTimezones(): TimezoneItem[] {
  const now = new Date();
  const browserTz = getBrowserTimezone();
  const browserOffset = -now.getTimezoneOffset();
  const browserOffsetStr = formatOffsetString(browserOffset);
  const browserAbbr = getTimezoneAbbr(browserTz, now);
  const browserCity = browserTz.includes("/")
    ? browserTz.split("/")[1].replace(/_/g, " ")
    : browserTz;

  const browserItem: TimezoneItem = {
    id: "browser",
    name: "Browser Time",
    fullName: `Browser Time (${browserCity}, ${browserAbbr})`,
    region: "Default",
    offsetStr: browserOffsetStr,
    offsetMinutes: browserOffset,
    abbr: browserAbbr,
    searchKey: `browser time default local ${browserTz} ${browserCity} ${browserAbbr} ${browserOffsetStr}`.toLowerCase(),
  };

  const utcItem: TimezoneItem = {
    id: "UTC",
    name: "Coordinated Universal Time",
    fullName: "Coordinated Universal Time (UTC, GMT)",
    region: "Global",
    offsetStr: "UTC+00:00",
    offsetMinutes: 0,
    abbr: "UTC",
    searchKey: "utc gmt coordinated universal time z 00:00".toLowerCase(),
  };

  return [browserItem, utcItem];
}

/**
 * Searches and groups timezones by region based on the search query.
 */
export function searchTimezones(query: string = ""): {
  quick: TimezoneItem[];
  groups: { region: string; items: TimezoneItem[] }[];
} {
  const normalizedQuery = query.trim().toLowerCase();
  const quick = getQuickTimezones();
  const all = getAllTimezoneItems();

  if (!normalizedQuery) {
    // Group all by region
    const groupMap = new Map<string, TimezoneItem[]>();
    for (const item of all) {
      const list = groupMap.get(item.region) ?? [];
      list.push(item);
      groupMap.set(item.region, list);
    }
    const groups = Array.from(groupMap.entries()).map(([region, items]) => ({
      region,
      items,
    }));
    return { quick, groups };
  }

  // Filter quick items
  const matchedQuick = quick.filter((item) =>
    item.searchKey.includes(normalizedQuery),
  );

  // Filter all items
  const matchedAll = all.filter((item) =>
    item.searchKey.includes(normalizedQuery),
  );

  const groupMap = new Map<string, TimezoneItem[]>();
  for (const item of matchedAll) {
    const list = groupMap.get(item.region) ?? [];
    list.push(item);
    groupMap.set(item.region, list);
  }
  const groups = Array.from(groupMap.entries()).map(([region, items]) => ({
    region,
    items,
  }));

  return { quick: matchedQuick, groups };
}
