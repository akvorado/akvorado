<template>
  <div class="container mx-auto">
    <ResizeRow
      :slider-width="10"
      :height="graphHeight"
      width="auto"
      slider-bg-color="#eee3"
      slider-bg-hover-color="#ccc3"
    >
      <VisualizeGraph :data="fetchedData" />
    </ResizeRow>
    <VisualizeTable :data="fetchedData" />
  </div>
</template>

<script setup>
import { ref, watch } from "vue";
import { notify } from "notiwind";
import { Date as SugarDate } from "sugar-date";
import { ResizeRow } from "vue-resizer";
import VisualizeTable from "../components/VisualizeTable.vue";
import VisualizeGraph from "../components/VisualizeGraph.vue";

const graphHeight = ref(500);

const request = ref({
  start: "6 hours ago",
  end: "now",
  points: 100,
  limit: 10,
  dimensions: ["SrcAS", "ExporterName"],
  filter: {
    operator: "all",
    rules: [
      {
        column: "InIfBoundary",
        operator: "=",
        value: "external",
      },
    ],
  },
});
const fetchedData = ref({});

watch(
  request,
  async () => {
    let body = { ...request.value };
    body.start = SugarDate.create(body.start);
    body.end = SugarDate.create(body.end);
    const response = await fetch("/api/v0/console/graph", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      notify(
        {
          group: "top",
          kind: "error",
          title: "Unable to fetch data",
          text: `While retrieving data, got a fatal error.`,
        },
        60000
      );
      return;
    }
    const data = await response.json();
    data.dimensions = body.dimensions;
    fetchedData.value = data;
  },
  { immediate: true }
);
</script>
