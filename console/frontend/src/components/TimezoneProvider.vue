<!-- SPDX-FileCopyrightText: 2026 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <slot></slot>
</template>

<script lang="ts" setup>
import {
  computed,
  provide,
  inject,
  type InjectionKey,
  type Ref,
  type ComputedRef,
} from "vue";
import { useStorage } from "@vueuse/core";
import {
  getBrowserTimezone,
  getTimezoneOffsetMinutes,
  formatOffsetString,
  getTimezoneAbbr,
  shiftDateForEcharts,
  unshiftDateFromEcharts,
  formatDateInTimezone,
} from "@/utils/timezone";

const timezone = useStorage("akvorado-timezone", "browser");

const browserTimezone = getBrowserTimezone();

const resolvedTimezone = computed(() =>
  timezone.value === "browser" ? browserTimezone : timezone.value,
);

const isUTC = computed(
  () =>
    timezone.value === "UTC" ||
    resolvedTimezone.value === "UTC" ||
    resolvedTimezone.value === "Etc/UTC",
);

const isBrowser = computed(() => timezone.value === "browser");

const timezoneOffsetLabel = computed(() =>
  formatOffsetString(getTimezoneOffsetMinutes(resolvedTimezone.value)),
);

const timezoneAbbr = computed(() => getTimezoneAbbr(resolvedTimezone.value));

const displayLabel = computed(() => {
  if (timezone.value === "browser") {
    const city = browserTimezone.includes("/")
      ? browserTimezone.split("/")[1].replace(/_/g, " ")
      : browserTimezone;
    return `Browser Time (${city}, ${timezoneOffsetLabel.value})`;
  }
  if (timezone.value === "UTC" || timezone.value === "Etc/UTC") {
    return "Coordinated Universal Time (UTC)";
  }
  return `${timezone.value} (${timezoneOffsetLabel.value})`;
});

const setTimezone = (tz: string) => {
  timezone.value = tz;
};

const shiftDate = (date: Date | string | number): Date =>
  shiftDateForEcharts(date, timezone.value);

const unshiftDate = (date: Date | string | number): Date =>
  unshiftDateFromEcharts(date, timezone.value);

const formatDate = (
  date: Date | string | number,
  style: "full" | "short" | "timeOnly" | "iso" = "full",
): string => formatDateInTimezone(date, timezone.value, style);

provide(TimezoneKey, {
  timezone,
  setTimezone,
  browserTimezone,
  resolvedTimezone,
  isUTC,
  isBrowser,
  timezoneOffsetLabel,
  timezoneAbbr,
  displayLabel,
  shiftDate,
  unshiftDate,
  formatDate,
});
</script>

<script lang="ts">
export interface TimezoneContext {
  timezone: Ref<string>;
  setTimezone: (tz: string) => void;
  browserTimezone: string;
  resolvedTimezone: ComputedRef<string>;
  isUTC: ComputedRef<boolean>;
  isBrowser: ComputedRef<boolean>;
  timezoneOffsetLabel: ComputedRef<string>;
  timezoneAbbr: ComputedRef<string>;
  displayLabel: ComputedRef<string>;
  shiftDate: (date: Date | string | number) => Date;
  unshiftDate: (date: Date | string | number) => Date;
  formatDate: (
    date: Date | string | number,
    style?: "full" | "short" | "timeOnly" | "iso",
  ) => string;
}

export const TimezoneKey: InjectionKey<TimezoneContext> = Symbol("TimezoneKey");

export function useTimezone(): TimezoneContext {
  const ctx = inject(TimezoneKey);
  if (!ctx) {
    throw new Error("useTimezone must be used within a TimezoneProvider");
  }
  return ctx;
}
</script>
