<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div>
    <div
      class="flex items-end justify-between border-b border-gray-200 text-center text-sm font-medium text-gray-500 dark:border-gray-700 dark:text-gray-400"
      :class="{ 'print:hidden': !axes || axes.length <= 1 }"
    >
      <!-- Axis selection -->
      <ul class="flex flex-wrap">
        <template v-if="axes && axes.length > 1">
          <li v-for="{ id: axis, name } in axes" :key="axis" class="mr-2">
            <button
              class="pointer-cursor inline-block rounded-t-lg border-b-2 border-transparent p-4 hover:border-gray-300 hover:text-gray-600 dark:hover:text-gray-300"
              :class="{
                'active border-blue-600 text-blue-600 dark:border-blue-500 dark:text-blue-500':
                  displayedAxis === axis,
              }"
              :aria-current="displayedAxis === axis ? 'page' : undefined"
              @click="selectedAxis = axis"
            >
              {{ name }}
            </button>
          </li>
        </template>
      </ul>
      <button
        v-if="table"
        type="button"
        title="Download the table as CSV"
        class="mb-1 flex cursor-pointer items-center gap-1 rounded px-2 py-1 text-xs hover:bg-gray-200 hover:text-gray-700 print:hidden dark:hover:bg-gray-700 dark:hover:text-gray-200"
        @click="downloadCSV"
      >
        <DownloadIcon class="h-4 w-4" aria-hidden="true" />
        CSV
      </button>
    </div>
    <!-- Table -->
    <div
      class="relative overflow-x-auto shadow-md dark:shadow-white/10 sm:rounded-lg"
    >
      <table
        v-if="table"
        class="w-full max-w-full text-left text-sm text-gray-700 dark:text-gray-200"
      >
        <thead class="bg-gray-50 text-xs uppercase dark:bg-gray-700">
          <tr>
            <th
              scope="col"
              :class="{ 'px-6 py-2': table.rows.some((r) => r.color) }"
            ></th>
            <th
              v-for="(column, idx) in table.columns"
              :key="column.name"
              scope="col"
              class="px-6 py-2"
              :class="column.classNames"
              :aria-sort="
                sort?.column === idx
                  ? sort.descending
                    ? 'descending'
                    : 'ascending'
                  : undefined
              "
            >
              <button
                type="button"
                class="inline-flex cursor-pointer items-center gap-1 uppercase hover:text-gray-900 dark:hover:text-white"
                :title="`Sort by ${column.name}`"
                @click="toggleSort(idx)"
              >
                {{ column.name }}
                <component
                  :is="sort.descending ? ChevronDownIcon : ChevronUpIcon"
                  v-if="sort?.column === idx"
                  class="h-4 w-4"
                  aria-hidden="true"
                />
              </button>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in sortedRows"
            :key="row.index"
            class="border-b border-gray-200 odd:bg-white even:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 odd:dark:bg-gray-800 even:dark:bg-gray-700"
            :class="{ 'text-gray-400 dark:text-gray-500': isHidden(row.index) }"
            @pointerenter="highlight(row.index)"
            @pointerleave="highlight(null)"
          >
            <th scope="row">
              <div v-if="row.color" class="px-6 py-2 text-right font-medium">
                <button
                  type="button"
                  class="block w-5 cursor-pointer rounded border-2"
                  :style="{
                    backgroundColor: isHidden(row.index)
                      ? 'transparent'
                      : row.color,
                    borderColor: row.color,
                    printColorAdjust: 'exact',
                  }"
                  :title="
                    isHidden(row.index)
                      ? 'Show this series in the graph'
                      : 'Hide this series in the graph'
                  "
                  :aria-pressed="!isHidden(row.index)"
                  @click="emit('toggle', row.index)"
                >
                  &nbsp;
                </button>
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

<script lang="ts" setup>
import { computed, inject, ref, watch } from "vue";
import { uniqWith, isEqual, findIndex, toPairs } from "lodash-es";
import {
  ChevronDownIcon,
  ChevronUpIcon,
  DownloadIcon,
} from "@heroicons/vue/solid";
import {
  formatXps,
  dataColor,
  dataColorGrey,
  reverseDimension,
  toCSV,
} from "@/utils";
import { ThemeKey } from "@/components/ThemeProvider.vue";
import { ServerConfigKey } from "@/components/ServerConfigProvider.vue";
import type { GraphLineHandlerResult, GraphSankeyHandlerResult } from ".";
const { isDark } = inject(ThemeKey)!;
const serverConfiguration = inject(ServerConfigKey)!;

const props = defineProps<{
  data: GraphLineHandlerResult | GraphSankeyHandlerResult | null;
  hidden?: number[];
}>();
const emit = defineEmits<{
  highlighted: [index: number | null];
  toggle: [index: number];
}>();
const isHidden = (index: number) => !!props.hidden?.includes(index);

const highlight = (index: number | null) => {
  if (
    props.data == null ||
    props.data.graphType == "sankey" ||
    props.data.graphType == "heatmap"
  ) {
    emit("highlighted", null);
    return;
  }
  // A hidden series has nothing to highlight.
  emit("highlighted", index !== null && isHidden(index) ? null : index);
};
const axes = computed(() => {
  if (!props.data) return null;
  return toPairs(props.data["axis-names"])
    .map(([k, v]) => ({ id: Number(k), name: v }))
    .filter(({ id }) => [1, 2].includes(id))
    .sort(({ id: id1 }, { id: id2 }) => id1 - id2);
});
const selectedAxis = ref(1);
const displayedAxis = computed(() => {
  if (!axes.value) return null;
  return axes.value.some((axis) => axis.id === selectedAxis.value)
    ? selectedAxis.value
    : 1;
});
// The reverse axis groups on the opposite dimensions: SrcAS becomes DstAS and
// so on.
const dimensions = computed(() => {
  const selected = props.data?.dimensions ?? [];
  if (displayedAxis.value !== 2) return selected;
  const known = serverConfiguration.value?.dimensions ?? [];
  return selected.map((dimension) => reverseDimension(dimension, known));
});

type Cell = {
  value: string;
  raw: string | number;
  classNames?: string;
};
const table = computed(
  (): {
    columns: { name: string; unit?: string; classNames?: string }[];
    rows: {
      // Index of the row in the original data, used for highlighting.
      index: number;
      values: Cell[];
      color?: string;
    }[];
  } | null => {
    const theme = isDark.value ? "dark" : "light";
    const data = props.data;
    if (data === null) return null;
    const unit = ["inl2%", "outl2%"].includes(data.units)
      ? "%"
      : data.units.slice(-3);
    const formatValue = (v: number): string =>
      unit === "%" ? `${v.toFixed(0)}%` : `${formatXps(v)}${unit}`;
    const statCell = (v: number): Cell => ({
      value: formatValue(v),
      raw: v,
      classNames: "text-right tabular-nums",
    });
    const isBps = data.units.endsWith("bps");
    const totalUnit = isBps ? "B" : data.units.endsWith("pps") ? "p" : "f";
    const formatTotal = (v: number): string =>
      `${formatXps(isBps ? v / 8 : v)}${totalUnit}`;
    const dimensionColumns = dimensions.value.map((col) => ({
      name: col.replace(/([a-z])([A-Z])/g, "$1 $2"),
    }));
    const dimensionCells = (row: string[]): Cell[] =>
      row.map((r) => ({ value: r, raw: r }));
    // The server does not compute shares for percentages.
    const shareColumns = data.share
      ? [{ name: "Share", unit: "%", classNames: "text-right" }]
      : [];
    const share = (idx: number): Cell[] =>
      data.share
        ? [
            {
              value: `${data.share[idx].toFixed(1)}%`,
              raw: data.share[idx],
              classNames: "text-right tabular-nums",
            },
          ]
        : [];
    if (
      data.graphType === "stacked" ||
      data.graphType === "stacked100" ||
      data.graphType === "lines" ||
      data.graphType === "grid" ||
      data.graphType === "heatmap"
    ) {
      const uniqRows = uniqWith(data.rows, isEqual),
        uniqRowIndex = (row: string[]) =>
          findIndex(uniqRows, (orow) => isEqual(row, orow));
      const statColumn = (name: string) => ({
        name,
        unit,
        classNames: "text-right",
      });
      return {
        columns: [
          ...dimensionColumns,
          statColumn("Min"),
          statColumn("Max"),
          statColumn("Last"),
          statColumn("Average"),
          ...shareColumns,
          statColumn("~95th"),
          ...(data.total
            ? [
                {
                  name: "Total",
                  unit: { B: "bytes", p: "packets", f: "flows" }[totalUnit],
                  classNames: "text-right",
                },
              ]
            : []),
        ],
        rows: data.rows
          .map((row, idx) => {
            const color = row.some((name) => name === "Other")
              ? dataColorGrey
              : dataColor;
            return {
              index: idx,
              values: [
                ...dimensionCells(row),
                ...[
                  data.min[idx],
                  data.max[idx],
                  data.last[idx],
                  data.average[idx],
                ].map(statCell),
                ...share(idx),
                statCell(data["95th"][idx]),
                ...(data.total
                  ? [
                      {
                        value: formatTotal(data.total[idx]),
                        raw: isBps ? data.total[idx] / 8 : data.total[idx],
                        classNames: "text-right tabular-nums",
                      },
                    ]
                  : []),
              ],
              color:
                data.graphType === "heatmap"
                  ? undefined
                  : color(uniqRowIndex(row), false, theme),
            };
          })
          .filter(({ index }) => data.axis[index] == displayedAxis.value),
      };
    } else if (data.graphType === "sankey") {
      return {
        columns: [
          ...dimensionColumns,
          { name: "Average", unit, classNames: "text-right" },
          ...shareColumns,
        ],
        rows: (data.rows ?? [])
          .map((row, idx) => ({
            index: idx,
            values: [
              ...dimensionCells(row),
              statCell(data.xps[idx]),
              ...share(idx),
            ],
          }))
          .filter(({ index }) => data.axis[index] === displayedAxis.value),
      };
    }
    return null;
  },
);

// Rows are displayed in the order returned by the server, unless a column is
// selected for sorting. Clicking a column header cycles between the natural
// order of the column, the reverse order, and no sorting.
const sort = ref<{ column: number; descending: boolean } | null>(null);
watch(
  () => table.value?.columns.map(({ name }) => name),
  (columns, previous) => {
    if (!isEqual(columns, previous)) sort.value = null;
  },
);
const toggleSort = (column: number) => {
  const numeric = table.value?.columns[column].unit !== undefined;
  if (sort.value?.column !== column) {
    sort.value = { column, descending: numeric };
  } else if (sort.value.descending === numeric) {
    sort.value = { column, descending: !numeric };
  } else {
    sort.value = null;
  }
};
const sortedRows = computed(() => {
  const rows = table.value?.rows ?? [];
  const s = sort.value;
  if (s === null) return rows;
  const direction = s.descending ? -1 : 1;
  return [...rows].sort((r1, r2) => {
    const v1 = r1.values[s.column].raw,
      v2 = r2.values[s.column].raw;
    const cmp =
      typeof v1 === "number" && typeof v2 === "number"
        ? v1 - v2
        : String(v1).localeCompare(String(v2), undefined, { numeric: true });
    return direction * cmp;
  });
});

const downloadCSV = () => {
  if (!table.value || !props.data) return;
  const header = table.value.columns.map(({ name, unit }) =>
    unit ? `${name} (${unit})` : name,
  );
  const rows = sortedRows.value.map(({ values }) =>
    values.map(({ raw }) => raw),
  );
  const blob = new Blob([toCSV([header, ...rows])], {
    type: "text/csv;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `akvorado-${props.data.start}-${props.data.end}.csv`.replace(
    /\.\d+Z|:/g,
    "",
  );
  link.click();
  URL.revokeObjectURL(url);
};
</script>
