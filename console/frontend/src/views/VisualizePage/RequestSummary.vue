<template>
  <div
    class="z-10 hidden h-7 w-full items-center gap-3 whitespace-nowrap border-b border-gray-300 bg-gray-100 px-4 text-xs text-gray-400 dark:border-slate-700 dark:bg-slate-800 dark:text-gray-500 lg:flex"
  >
    <span v-if="request.graphType" class="shrink-0">
      <ChartPieIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ request.graphType }}</span>
    </span>
    <span v-if="request.start && request.end" class="shrink-0">
      <CalendarIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ start }} — {{ end }}</span>
    </span>
    <span
      v-if="request.dimensions && request.dimensions.length > 0"
      class="truncate"
      :title="request.dimensions.join(', ')"
    >
      <AdjustmentsIcon class="inline h-4 px-1 align-middle" />
      <span class="truncate align-middle">{{
        request.dimensions.join(", ")
      }}</span>
    </span>
    <span v-if="request.filter" class="truncate" :title="request.filter">
      <FilterIcon class="inline h-4 px-1 align-middle" />
      <span class="max-w-xs align-middle">{{ request.filter }}</span>
    </span>
    <span v-if="request.limit" class="shrink-0">
      <ArrowUpIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ request.limit }}</span>
    </span>
    <span v-if="request.units" class="shrink-0">
      <HashtagIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{
        { bps: "ᵇ⁄ₛ", pps: "ᵖ⁄ₛ" }[request.units] || requests.units
      }}</span>
    </span>
  </div>
</template>

<script setup>
const props = defineProps({
  request: {
    type: Object,
    required: true,
  },
});

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

const start = computed(() => SugarDate(props.request.start).long());
const end = computed(() =>
  SugarDate(props.request.end).format(
    props.request.start.toDateString() === props.request.end.toDateString()
      ? "%X"
      : "{long}"
  )
);

// Also set title
const title = inject("title");
const computedTitle = computed(() =>
  [
    props.request.graphType,
    props.request?.dimensions?.join(","),
    props.request.filter,
    start.value,
    end.value,
  ]
    .filter((e) => !!e)
    .join(" · ")
);
watch(computedTitle, (t) => title.set(t));
</script>
