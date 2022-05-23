<template>
  <div
    class="relative my-3 overflow-x-auto shadow-md dark:shadow-white/10 sm:rounded-lg"
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
          @pointerenter="highlightEnabled && $emit('highlighted', index)"
          @pointerleave="$emit('highlighted', null)"
        >
          <th scope="row">
            <div v-if="row.color" class="px-6 py-2 text-right font-medium">
              <div
                class="w-5 cursor-pointer"
                :style="{ backgroundColor: row.color }"
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
</template>

<script setup>
const props = defineProps({
  data: {
    type: Object,
    default: null,
  },
});
defineEmits(["highlighted"]);

import { computed, inject } from "vue";
import { formatBps, dataColor, dataColorGrey } from "@/utils";
import { graphTypes } from "./constants";
const { isDark } = inject("theme");
const { stacked, lines, grid, sankey } = graphTypes;

const highlightEnabled = computed(() =>
  [stacked, lines, grid].includes(props.data?.graphType)
);
const table = computed(() => {
  const theme = isDark.value ? "dark" : "light";
  const data = props.data || {};
  if ([stacked, lines, grid].includes(data.graphType)) {
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
      ],
      rows:
        data.rows?.map((rows, idx) => {
          const color = rows.some((name) => name === "Other")
            ? dataColorGrey
            : dataColor;
          return {
            values: [
              // Dimensions
              ...rows.map((r) => ({ value: r })),
              // Stats
              ...[data.min[idx], data.max[idx], data.average[idx]].map((d) => ({
                value: formatBps(d) + "bps",
                classNames: "text-right tabular-nums",
              })),
            ],
            color: color(idx, false, theme),
          };
        }) || [],
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
      rows: data.rows?.map((rows, idx) => ({
        values: [
          // Dimensions
          ...rows.map((r) => ({ value: r })),
          // Average
          {
            value: formatBps(data.bps[idx]) + "bps",
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
