<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div>
    <h1 class="font-semibold leading-relaxed">{{ title }}</h1>
    <div class="h-[200px]">
      <v-chart :option="options" :theme="isDark ? 'dark' : null" autoresize />
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  refresh: {
    type: Number,
    required: true,
  },
  what: {
    type: String,
    required: true,
  },
  title: {
    type: String,
    required: true,
  },
});

import { computed, inject } from "vue";
import { useFetch } from "@vueuse/core";
import { use } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { PieChart } from "echarts/charts";
import { TooltipComponent, LegendComponent } from "echarts/components";
import VChart from "vue-echarts";
import { dataColor, dataColorGrey } from "../../utils";
const { isDark } = inject("theme");

use([CanvasRenderer, PieChart, TooltipComponent, LegendComponent]);

const url = computed(
  () => `/api/v0/console/widget/top/${props.what}?${props.refresh}`
);
const { data } = useFetch(url, { refetch: true }).get().json();
const options = computed(() => ({
  darkMode: isDark.value,
  backgroundColor: "transparent",
  tooltip: {
    trigger: "item",
    confine: true,
    valueFormatter(value) {
      return value.toFixed(2) + "%";
    },
  },
  legend: {
    orient: "horizontal",
    bottom: "bottom",
    itemGap: 5,
    itemWidth: 14,
    itemHeight: 14,
    textStyle: {
      fontSize: 10,
      color: isDark.value ? "#eee" : "#111",
    },
    formatter(name) {
      return name.split(": ")[0];
    },
  },
  series: [
    {
      type: "pie",
      label: { show: false },
      center: ["50%", "40%"],
      radius: "60%",
      data: [
        ...(data.value?.top || [])
          .filter(({ percent }) => percent > 0)
          .map(({ name, percent }) => ({
            name,
            value: percent,
          })),
        {
          name: "Others",
          value: Math.max(
            100 - (data.value?.top || []).reduce((c, n) => c + n.percent, 0),
            0
          ),
        },
      ].filter(({ value }) => value > 0.05),
      itemStyle: {
        color({ name, dataIndex }) {
          const theme = isDark.value ? "dark" : "light";
          if (name === "Others") {
            return dataColorGrey(0, false, theme);
          }
          return dataColor(dataIndex, false, theme);
        },
      },
    },
  ],
}));
</script>
