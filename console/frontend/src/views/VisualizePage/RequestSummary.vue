<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div
    v-if="request"
    class="z-10 flex w-full flex-wrap items-center gap-x-3 whitespace-nowrap border-b border-gray-300 bg-gray-100 px-4 text-xs text-gray-400 dark:border-slate-700 dark:bg-slate-800 dark:text-gray-500 sm:flex-nowrap print:sm:flex-wrap"
  >
    <span class="shrink-0 py-0.5">
      <CalendarIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ start }} — {{ end }}</span>
    </span>
    <span class="shrink-0 py-0.5">
      <ChartPieIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ graphTypes[request.graphType] }}</span>
    </span>
    <span class="min-w-[4 shrink-0 py-0.5">
      <ArrowUpIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ request.limit }}</span>
    </span>
    <span class="min-w-[4 shrink-0 py-0.5">
      <HashtagIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{
        {
          l3bps: "L3ᵇ⁄ₛ",
          l2bps: "L2ᵇ⁄ₛ",
          "inl2%": "→L2%",
          "outl2%": "L2%→",
          pps: "ᵖ⁄ₛ",
        }[request.units]
      }}</span>
    </span>
    <span
      v-if="request.dimensions.length > 0"
      class="min-w-[3rem] truncate py-0.5"
      :title="request.dimensions.join(', ')"
    >
      <AdjustmentsIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ request.dimensions.join(", ") }}</span>
    </span>
    <span
      v-if="request.filter"
      class="min-w-[3rem] truncate py-0.5"
      :title="request.filter"
    >
      <FilterIcon class="inline h-4 px-1 align-middle" />
      <span class="max-w-xs align-middle">{{ request.filter }}</span>
    </span>
  </div>
</template>

<script lang="ts" setup>
import { computed, watch, inject } from "vue";
import {
  ChartPieIcon,
  CalendarIcon,
  AdjustmentsIcon,
  ArrowUpIcon,
  FilterIcon,
  HashtagIcon,
} from "@heroicons/vue/solid";
import { Date as SugarDate } from "sugar-date";
import type { ModelType } from "./OptionsPanel.vue";
import { graphTypes } from "./graphtypes";
import { TitleKey } from "@/components/TitleProvider.vue";

const props = defineProps<{ request: ModelType }>();

const start = computed(() =>
  props.request ? SugarDate(props.request.start).long() : null
);
const end = computed(() => {
  if (props.request === null) return null;
  return SugarDate(props.request.end).format(
    SugarDate(props.request.start).toDateString().raw ===
      SugarDate(props.request.end).toDateString().raw
      ? "%X"
      : "{long}"
  );
});

// Also set title
const title = inject(TitleKey)!;
const computedTitle = computed(() =>
  [
    props.request ? graphTypes[props.request?.graphType] : null,
    props.request?.dimensions?.join(","),
    props.request?.filter,
    start.value,
    end.value,
  ]
    .filter((e) => !!e)
    .join(" · ")
);
watch(computedTitle, (t) => title.set(t));
</script>
