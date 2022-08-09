<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div>
    <!-- Axis selection -->
    <div
      v-if="axes.length > 1"
      class="border-b border-gray-200 text-center text-sm font-medium text-gray-500 dark:border-gray-700 dark:text-gray-400"
    >
      <ul class="-mb-px flex flex-wrap">
        <li v-for="{ id: axis, name } in axes" :key="axis" class="mr-2">
          <button
            class="pointer-cursor inline-block rounded-t-lg border-b-2 border-transparent p-4 hover:border-gray-300 hover:text-gray-600 dark:hover:text-gray-300"
            :class="{
              'active border-blue-600 text-blue-600 dark:border-blue-500 dark:text-blue-500':
                displayedAxis === axis,
            }"
            :aria-current="displayedAxis === axis ? 'page' : null"
            @click="selectedAxis = axis"
          >
            {{ name }}
          </button>
        </li>
      </ul>
    </div>
    <!-- Table -->
    <div
      class="relative overflow-x-auto shadow-md dark:shadow-white/10 sm:rounded-lg"
    >
      <table
        class="w-full max-w-full text-left text-sm text-gray-700 dark:text-gray-200"
      >
        <thead class="bg-gray-50 text-xs uppercase dark:bg-gray-700">
          <tr>
            <th
              scope="col"
              :class="{ 'px-6 py-2': table.rows.some((r) => r.color) }"
            ></th>
            <th
              v-for="column in table.columns"
              :key="column.name"
              scope="col"
              class="px-6 py-2"
              :class="column.classNames"
            >
              {{ column.name }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(row, index) in table.rows"
            :key="index"
            class="border-b odd:bg-white even:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 odd:dark:bg-gray-800 even:dark:bg-gray-700"
            @pointerenter="highlight(index)"
            @pointerleave="highlight(null)"
          >
            <th scope="row">
              <div v-if="row.color" class="px-6 py-2 text-right font-medium">
                <div
                  class="w-5 cursor-pointer rounded"
                  :style="{
                    backgroundColor: row.color,
                    printColorAdjust: 'exact',
                  }"
                >
                  &nbsp;
                </div>
              </div>
            </th>
            <td
              v-for="(value, idx) in row.values"
              :key="idx"
              class="px-6 py-2"
              :class="value.classNames"
            >
              {{ value.value }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  data: {
    type: Object,
    default: null,
  },
});
const emit = defineEmits(["highlighted"]);

import { computed, inject, ref } from "vue";
import { formatXps, dataColor, dataColorGrey } from "@/utils";
import { graphTypes } from "./constants";
const { isDark } = inject("theme");
const { stacked, lines, grid, sankey } = graphTypes;

import { uniq, uniqWith, isEqual, findIndex, takeWhile } from "lodash-es";

const highlight = (index) => {
  if (index === null) {
    emit("highlighted", null);
    return;
  }
  if (![stacked, lines, grid].includes(props.data?.graphType)) return;
  // The index provided is the one in the filtered data. We want the original index.
  const originalIndex = takeWhile(
    props.data.rows,
    (() => {
      let count = 0;
      return (_, idx) =>
        props.data.axis[idx] != displayedAxis.value || count++ < index;
    })()
  ).length;
  emit("highlighted", originalIndex);
};
const axes = computed(() =>
  uniq(props.data.axis ?? []).map((axis) => ({
    id: axis,
    name: { 1: "Direct", 2: "Reverse" }[axis] ?? "Unknown",
  }))
);
const selectedAxis = ref(1);
const displayedAxis = computed(() =>
  axes.value.some((axis) => axis.id === selectedAxis.value)
    ? selectedAxis.value
    : 1
);
const table = computed(() => {
  const theme = isDark.value ? "dark" : "light";
  const data = props.data || {};
  if ([stacked, lines, grid].includes(data.graphType)) {
    const uniqRows = uniqWith(data.rows, isEqual),
      uniqRowIndex = (row) => findIndex(uniqRows, (orow) => isEqual(row, orow));
    return {
      columns: [
        // Dimensions
        ...(data.dimensions?.map((col) => ({
          name: col.replace(/([a-z])([A-Z])/g, "$1 $2"),
        })) || []),
        // Stats
        { name: "Min", classNames: "text-right" },
        { name: "Max", classNames: "text-right" },
        { name: "Average", classNames: "text-right" },
        { name: "~95th", classNames: "text-right" },
      ],
      rows:
        data.rows
          ?.map((row, idx) => {
            const color = row.some((name) => name === "Other")
              ? dataColorGrey
              : dataColor;
            return {
              values: [
                // Dimensions
                ...row.map((r) => ({ value: r })),
                // Stats
                ...[
                  data.min[idx],
                  data.max[idx],
                  data.average[idx],
                  data["95th"][idx],
                ].map((d) => ({
                  value: formatXps(d) + data.units.slice(-3),
                  classNames: "text-right tabular-nums",
                })),
              ],
              color: color(uniqRowIndex(row), false, theme),
            };
          })
          .filter((_, idx) => data.axis[idx] == displayedAxis.value) || [],
    };
  }
  if ([sankey].includes(data.graphType)) {
    return {
      columns: [
        // Dimensions
        ...(data.dimensions?.map((col) => ({
          name: col.replace(/([a-z])([A-Z])/, "$1 $2"),
        })) || []),
        // Average
        { name: "Average", classNames: "text-right" },
      ],
      rows: data.rows?.map((row, idx) => ({
        values: [
          // Dimensions
          ...row.map((r) => ({ value: r })),
          // Average
          {
            value: formatXps(data.xps[idx]) + data.units.slice(-3),
            classNames: "text-right tabular-nums",
          },
        ],
      })),
    };
  }
  return {
    columns: [],
    rows: [],
  };
});
</script>
