<template>
  <div class="relative my-3 overflow-x-auto shadow-md sm:rounded-lg">
    <table
      class="w-full max-w-full text-left text-sm text-gray-500 dark:text-gray-400"
    >
      <thead
        class="bg-gray-50 text-xs uppercase text-gray-700 dark:bg-gray-700 dark:text-gray-400"
      >
        <tr>
          <th scope="col" class="px-6 py-2"></th>
          <th
            v-for="column in table.columns"
            :key="column"
            scope="col"
            class="px-6 py-2"
          >
            {{ column }}
          </th>
          <th scope="col" class="px-6 py-2 text-right">Min</th>
          <th scope="col" class="px-6 py-2 text-right">Max</th>
          <th scope="col" class="px-6 py-2 text-right">Average</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="(row, index) in table.rows"
          :key="index"
          class="border-b odd:bg-white even:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 odd:dark:bg-gray-800 even:dark:bg-gray-700"
          @pointerenter="$emit('highlighted', index)"
          @pointerleave="$emit('highlighted', null)"
        >
          <th
            scope="row"
            class="px-6 py-2 text-right font-medium text-gray-900 dark:text-white"
          >
            <div class="w-5 cursor-pointer" :style="row.style">&nbsp;</div>
          </th>
          <td
            v-for="dimension in row.dimensions"
            :key="dimension"
            class="px-6 py-2"
          >
            {{ dimension }}
          </td>
          <td class="px-6 py-2 text-right tabular-nums">
            {{ formatBps(row.min) }}bps
          </td>
          <td class="px-6 py-2 text-right tabular-nums">
            {{ formatBps(row.max) }}bps
          </td>
          <td class="px-6 py-2 text-right tabular-nums">
            {{ formatBps(row.average) }}bps
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { ref, watch, inject } from "vue";
import { formatBps, dataColor, dataColorGrey } from "../utils";
const { isDark } = inject("darkMode");

const props = defineProps({
  data: {
    type: Object,
    default: () => {},
  },
});
defineEmits(["highlighted"]);

const table = ref({
  columns: [],
  rows: [],
});

watch(
  () => [props.data, isDark()],
  ([data, isDark]) => {
    if (data.t === undefined) {
      return;
    }
    const theme = isDark ? "dark" : "light";
    table.value = {
      columns: data.dimensions,
      rows: data.rows.map((rows, idx) => {
        const color = rows.some((name) => name === "Other")
          ? dataColorGrey
          : dataColor;
        return {
          dimensions: rows,
          style: `background-color: ${color(idx, false, theme)}`,
          min: data.min[idx],
          max: data.max[idx],
          average: data.average[idx],
        };
      }),
    };
  },
  { immediate: true }
);
</script>
