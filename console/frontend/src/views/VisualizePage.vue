<template>
  <div class="flex h-full w-full flex-col lg:flex-row">
    <OptionsPanel
      v-model="state"
      :loading="isFetching"
      @cancel="canAbort && abort()"
    />
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
            :loading="isFetching"
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
import { ref, watch, computed } from "vue";
import { useFetch } from "@vueuse/core";
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
  limit: 10,
  dimensions: ["SrcAS", "ExporterName"],
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
  route,
  () => {
    const newState = decodeState(route.params.state);
    if (!isEqual(newState, state.value)) {
      state.value = newState;
    }
  },
  { immediate: true }
);
const encodedState = computed(() => encodeState(state.value));
watch(
  encodedState,
  () => {
    const routeTarget = {
      name: "VisualizeWithState",
      params: { state: encodedState.value },
    };
    if (route.name !== "VisualizeWithState") {
      router.replace(routeTarget);
    } else {
      router.push(routeTarget);
    }
  },
  { immediate: true, deep: true }
);

// Fetch data
const payload = computed(() => ({
  ...state.value,
  start: SugarDate.create(state.value.start),
  end: SugarDate.create(state.value.end),
}));
const { data, isFetching, aborted, abort, canAbort, error } = useFetch(
  "/api/v0/console/graph",
  {
    refetch: true,
  }
)
  .post(payload)
  .json();
const errorMessage = computed(
  () =>
    (error.value &&
      !aborted.value &&
      (data.value?.message || `Server returned an error: ${error.value}`)) ||
    ""
);
const fetchedData = computed(() =>
  error.value || aborted.value || data.value === null
    ? fetchedData.value
    : {
        ...data.value,
        dimensions: state.value.dimensions,
        start: state.value.start,
      }
);
</script>
