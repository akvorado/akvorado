<template>
  <div>
    <h1 class="font-semibold leading-relaxed">Top AS</h1>
    <div class="h-[200px]">
      <v-chart :option="option" autoresize />
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from "vue";
import { use } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { PieChart } from "echarts/charts";
import { TooltipComponent, LegendComponent } from "echarts/components";
import VChart from "vue-echarts";

use([CanvasRenderer, PieChart, TooltipComponent, LegendComponent]);

const props = defineProps({
  refresh: {
    type: Number,
    default: 0,
  },
});
const option = ref({
  tooltip: {
    trigger: "item",
    formatter: "{b}: {c}%",
  },
  legend: {
    orient: "horizontal",
    bottom: "bottom",
    itemGap: 5,
    itemWidth: 14,
    itemHeight: 14,
    backgroundColor: "rgba(255, 255, 255, 0.4)",
    textStyle: { fontSize: 10 },
    formatter(name) {
      if (name === "Others") {
        return "…";
      }
      return name.split(": ")[0];
    },
  },
  series: [
    {
      name: "Top AS",
      type: "pie",
      label: { show: false },
      center: ["50%", "40%"],
      radius: "60%",
      itemStyle: {
        color({ name, dataIndex }) {
          if (name === "Others") {
            return "#aaa";
          }
          console.log(option.value);
          return ["#5470c6", "#91cc75", "#fac858", "#ee6666", "#73c0de"][
            dataIndex % 5
          ];
        },
      },
      data: [],
    },
  ],
});

watch(
  () => props.refresh,
  async () => {
    const response = await fetch("/api/v0/console/widget/top/src-as");
    if (!response.ok) {
      // Keep current data
      return;
    }
    const data = await response.json();
    const totalPercent = data.top.reduce((c, n) => c + n.percent, 0);
    const newData = [
      ...data.top.map(({ name, percent }) => ({
        name,
        value: percent,
      })),
      {
        name: "Others",
        value: Math.max(100 - totalPercent, 0),
      },
    ];
    option.value.series[0].data = newData;
  },
  { immediate: true }
);
</script>
