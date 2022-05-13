<template>
  <div class="flex w-full flex-col lg:flex-row">
    <VisualizeOptions
      :state="state"
      @update="(updatedState) => (state = updatedState)"
    />
    <div class="mx-4 grow">
      <InfoBox
        v-if="errorTitle"
        kind="danger"
        :title="errorTitle"
        :content="errorContent"
      />
      <ResizeRow
        :slider-width="10"
        :height="graphHeight"
        width="auto"
        slider-bg-color="#eee1"
        slider-bg-hover-color="#ccc3"
      >
        <VisualizeGraph :data="fetchedData" />
      </ResizeRow>
      <VisualizeTable :data="fetchedData" />
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Date as SugarDate } from "sugar-date";
import { ResizeRow } from "vue-resizer";
import LZString from "lz-string";
import VisualizeTable from "../components/VisualizeTable.vue";
import VisualizeGraph from "../components/VisualizeGraph.vue";
import VisualizeOptions from "../components/VisualizeOptions.vue";
import InfoBox from "../components/InfoBox.vue";

const route = useRoute();
const router = useRouter();
const graphHeight = ref(500);
const fetchedData = ref({});

const decodeState = (serialized) => {
  try {
    if (!serialized) {
      console.debug("no state, return default state");
      return defaultState();
    }
    return JSON.parse(LZString.decompressFromBase64(serialized));
  } catch (error) {
    console.error("cannot decode state:", error);
    return defaultState();
  }
};
const encodeState = (state) => {
  return LZString.compressToBase64(JSON.stringify(state));
};

// Main state
const defaultState = () => ({
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
const state = ref({});

// Error handling
const errorTitle = ref("");
const errorContent = ref("");

watch(
  route,
  () => {
    const newState = decodeState(route.params.state);
    if (JSON.stringify(newState) !== JSON.stringify(state.value)) {
      state.value = newState;
    }
  },
  { immediate: true }
);
watch(
  state,
  async () => {
    errorTitle.value = "";
    let body = { ...state.value };
    body.start = SugarDate.create(body.start);
    body.end = SugarDate.create(body.end);
    const response = await fetch("/api/v0/console/graph", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      errorTitle.value = "Unable to fetch data!";
      errorContent.value = "While retrieving data, got a fatal error.";
      try {
        const data = await response.json();
        if (data.message) {
          errorContent.value = `The server told us: ${data.message}.`;
        }
      } catch (_) {
        // Do nothing
      }
      return;
    }
    const data = await response.json();
    data.dimensions = body.dimensions;
    fetchedData.value = data;
    router.push({
      name: "VisualizeWithState",
      params: { state: encodeState(state.value) },
    });
  },
  { immediate: true }
);
</script>
