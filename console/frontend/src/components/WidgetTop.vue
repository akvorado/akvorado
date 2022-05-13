<template>
  <div>
    <h1 class="font-semibold leading-relaxed">{{ title }}</h1>
    <div class="h-[200px]">
      <v-chart :option="option" autoresize />
    </div>
  </div>
</template>

<script setup>
import { ref, watch, inject } from "vue";
import { use } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { PieChart } from "echarts/charts";
import { TooltipComponent, LegendComponent } from "echarts/components";
import VChart from "vue-echarts";
import { dataColor, dataColorGrey } from "../utils/palette.js";
const { isDark } = inject("darkMode");

use([CanvasRenderer, PieChart, TooltipComponent, LegendComponent]);

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
const option = ref({
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
    textStyle: { fontSize: 10 },
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
      data: [],
    },
  ],
});

watch(
  isDark,
  (isDark) => {
    const theme = isDark ? "dark" : "light";
    option.value.darkMode = isDark;
    option.value.series[0].itemStyle = {
      color({ name, dataIndex }) {
        if (name === "Others") {
          return dataColorGrey(0, false, theme);
        }
        return dataColor(dataIndex, false, theme);
      },
    };
    option.value.legend.textStyle.color = isDark ? "#eee" : "#111";
  },
  { immediate: true }
);

watch(
  () => props.refresh,
  async () => {
    const response = await fetch("/api/v0/console/widget/top/" + props.what);
    if (!response.ok) {
      // Keep current data
      return;
    }
    const data = await response.json();
    const totalPercent = data.top.reduce((c, n) => c + n.percent, 0);
    const newData = [
      ...data.top
        .filter(({ percent }) => percent > 0)
        .map(({ name, percent }) => ({
          name,
          value: percent,
        })),
      {
        name: "Others",
        value: Math.max(100 - totalPercent, 0),
      },
    ].filter(({ value }) => value > 0.05);
    option.value.series[0].data = newData;
  },
  { immediate: true }
);
</script>
