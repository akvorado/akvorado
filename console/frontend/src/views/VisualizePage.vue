<template>
  <div class="flex h-full w-full flex-col lg:flex-row">
    <OptionsPanel
      v-model="state"
      :loading="isFetching"
      @cancel="canAbort && abort()"
    />
    <div class="grow overflow-y-auto">
      <RequestSummary :request="request" />
      <div class="mx-4 my-2">
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
            :loading="isFetching"
            :data="fetchedData"
            :highlight="highlightedSerie"
            @update-time-range="updateTimeRange"
          />
        </ResizeRow>
        <DataTable
          :data="fetchedData"
          class="my-2"
          @highlighted="(n) => (highlightedSerie = n)"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  routeState: {
    type: String,
    default: "",
  },
});

import { ref, watch, computed } from "vue";
import { useFetch } from "@vueuse/core";
import { useRouter, useRoute } from "vue-router";
import { Date as SugarDate } from "sugar-date";
import { ResizeRow } from "vue-resizer";
import LZString from "lz-string";
import InfoBox from "@/components/InfoBox.vue";
import DataTable from "./VisualizePage/DataTable.vue";
import DataGraph from "./VisualizePage/DataGraph.vue";
import OptionsPanel from "./VisualizePage/OptionsPanel.vue";
import RequestSummary from "./VisualizePage/RequestSummary.vue";
import { graphTypes } from "./VisualizePage/constants";
import isEqual from "lodash.isequal";

const graphHeight = ref(500);
const highlightedSerie = ref(null);

const updateTimeRange = ([start, end]) => {
  state.value.start = start.toISOString();
  state.value.end = end.toISOString();
};

// Main state
const defaultState = () => ({
  graphType: graphTypes.stacked,
  start: "6 hours ago",
  end: "now",
  points: 200,
  dimensions: ["SrcAS", "ExporterName"],
  limit: 10,
  filter: "InIfBoundary = external",
});
const state = ref({});

// Load data from URL
const route = useRoute();
const router = useRouter();
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
watch(
  () => props.routeState,
  () => {
    const newState = decodeState(props.routeState);
    if (!isEqual(newState, state.value)) {
      state.value = newState;
    }
  },
  { immediate: true }
);
const encodedState = computed(() => encodeState(state.value));

// Fetch data
const fetchedData = ref({});
const payload = computed(() => ({
  ...state.value,
  start: SugarDate.create(state.value.start),
  end: SugarDate.create(state.value.end),
}));
const request = ref({}); // Same as payload, but once request is successful
const { data, isFetching, aborted, abort, canAbort, error } = useFetch("", {
  beforeFetch(ctx) {
    // Add the URL. Not a computed value as if we change both payload
    // and URL, the query will be triggered twice.
    const { cancel } = ctx;
    const endpoint = {
      [graphTypes.stacked]: "graph",
      [graphTypes.lines]: "graph",
      [graphTypes.grid]: "graph",
      [graphTypes.sankey]: "sankey",
    };
    const url = endpoint[state.value.graphType];
    if (url === undefined) {
      cancel();
    }
    return {
      ...ctx,
      url: `/api/v0/console/${url}`,
    };
  },
  afterFetch(ctx) {
    // Update data. Not done in a computed value as we want to keep the
    // previous data in case of errors.
    const { data } = ctx;
    fetchedData.value = {
      ...data,
      dimensions: payload.value.dimensions,
      start: payload.value.start,
      end: payload.value.end,
      graphType: payload.value.graphType,
    };

    // Also update URL.
    const routeTarget = {
      name: "VisualizeWithState",
      params: { state: encodedState.value },
    };
    if (route.name !== "VisualizeWithState") {
      router.replace(routeTarget);
    } else {
      router.push(routeTarget);
    }

    // Keep current payload for state
    request.value = payload.value;

    return ctx;
  },
  refetch: true,
})
  .post(payload)
  .json();
const errorMessage = computed(
  () =>
    (error.value &&
      !aborted.value &&
      (data.value?.message || `Server returned an error: ${error.value}`)) ||
    ""
);
</script>
