<template>
  <div
    class="fixed z-10 hidden w-full gap-4 overflow-hidden whitespace-nowrap border-b border-gray-300 bg-gray-100 px-4 pt-1 pb-2 text-sm text-gray-400 dark:border-slate-700 dark:bg-slate-800 dark:text-gray-500 lg:flex"
  >
    <span v-if="request.graphType">
      <ChartPieIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ request.graphType }}</span>
    </span>
    <span v-if="request.start && request.end">
      <CalendarIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ start }} — {{ end }}</span>
    </span>
    <span v-if="request.dimensions && request.dimensions.length > 0">
      <AdjustmentsIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ request.dimensions.join(", ") }}</span>
    </span>
    <span v-if="request.limit">
      <ArrowUpIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ request.limit }}</span>
    </span>
    <span v-if="request.filter">
      <FilterIcon class="inline h-4 px-1 align-middle" />
      <span class="align-middle">{{ request.filter }}</span>
    </span>
  </div>
  <div class="hidden h-8 lg:block"></div>
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
} from "@heroicons/vue/solid";
import { Date as SugarDate } from "sugar-date";

const start = computed(() => SugarDate(props.request.start).long());
const end = computed(() => SugarDate(props.request.end).long());

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
