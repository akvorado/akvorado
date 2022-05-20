<template>
  <div class="flex h-full w-full flex-col lg:flex-row">
    <OptionsPanel v-model="state" />
    <div class="grow overflow-y-auto">
      <div class="mx-4">
        <InfoBox v-if="errorMessage" kind="danger">
          <strong>Unable to fetch data!&nbsp;</strong>{{ errorMessage }}
        </InfoBox>
        <ResizeRow
          :slider-width="10"
          :height="graphHeight"
          width="auto"
          slider-bg-color="#eee1"
          slider-bg-hover-color="#ccc3"
        >
          <DataGraph
            :loading="loading"
            :data="fetchedData"
            :graph-type="state.graphType"
            :highlight="highlightedSerie"
            @update-time-range="updateTimeRange"
          />
        </ResizeRow>
        <DataTable
          :data="fetchedData"
          @highlighted="(n) => (highlightedSerie = n)"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Date as SugarDate } from "sugar-date";
import { ResizeRow } from "vue-resizer";
import LZString from "lz-string";
import DataTable from "./VisualizePage/DataTable.vue";
import DataGraph from "./VisualizePage/DataGraph.vue";
import OptionsPanel from "./VisualizePage/OptionsPanel.vue";
import InfoBox from "@/components/InfoBox.vue";
import { graphTypes } from "./VisualizePage/constants";
import isEqual from "lodash.isequal";

const route = useRoute();
const router = useRouter();
const graphHeight = ref(500);
const fetchedData = ref({});
const loading = ref(false);
const highlightedSerie = ref(null);

const decodeState = (serialized) => {
  try {
    if (!serialized) {
      console.debug("no state, return default state");
      return defaultState();
    }
    return {
      ...defaultState(),
      ...JSON.parse(LZString.decompressFromBase64(serialized)),
    };
  } catch (error) {
    console.error("cannot decode state:", error);
    return defaultState();
  }
};
const encodeState = (state) => {
  return LZString.compressToBase64(
    JSON.stringify(state, Object.keys(state).sort())
  );
};

// Main state
const defaultState = () => ({
  graphType: "blah",
  start: "6 hours ago",
  end: "now",
  points: 200,
  limit: 10,
  dimensions: ["SrcAS", "ExporterName"],
  filter: "InIfBoundary = external",
});
const state = ref({});
const errorMessage = ref("");

const updateTimeRange = ([start, end]) => {
  state.value.start = start.toISOString();
  state.value.end = end.toISOString();
};

watch(
  route,
  () => {
    const newState = decodeState(route.params.state);
    if (!isEqual(newState, state.value)) {
      state.value = newState;
    }
  },
  { immediate: true }
);
watch(
  state,
  async (state) => {
    errorMessage.value = "";
    let body = { ...state };
    body.start = SugarDate.create(body.start);
    body.end = SugarDate.create(body.end);
    loading.value = true;
    try {
      const response = await fetch("/api/v0/console/graph", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!response.ok) {
        try {
          const data = await response.json();
          errorMessage.value = data.message;
        } catch (_) {
          errorMessage.value = `Server returned a ${response.status} error`;
        }
        return;
      }
      const data = await response.json();
      data.dimensions = body.dimensions;
      data.start = body.start;
      data.end = body.end;
      fetchedData.value = data;
    } finally {
      loading.value = false;
    }
    const routeTarget = {
      name: "VisualizeWithState",
      params: { state: encodeState(state) },
    };
    if (route.name !== "VisualizeWithState") {
      router.replace(routeTarget);
    } else {
      router.push(routeTarget);
    }
  },
  { immediate: true, deep: true }
);
</script>
