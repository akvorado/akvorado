<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="flex h-full w-full flex-col lg:flex-row">
    <OptionsPanel
      v-model="state"
      :loading="isFetching"
      class="print:hidden"
      @cancel="canAbort && abort()"
    />
    <div class="grow overflow-y-auto">
      <LoadingOverlay :loading="isFetching">
        <RequestSummary :request="request" />
        <div class="mx-4 my-2">
          <InfoBox v-if="errorMessage" kind="error">
            <strong>Unable to fetch data!&nbsp;</strong>{{ errorMessage }}
          </InfoBox>
          <ResizeRow
            :slider-width="10"
            :height="graphHeight"
            width="auto"
            slider-bg-color="#eee1"
            slider-bg-hover-color="#ccc3"
            class="break-inside-avoid-page"
          >
            <DataGraph
              :data="fetchedData"
              :highlight="highlightedSerie"
              @update:time-range="updateTimeRange"
            />
          </ResizeRow>
          <DataTable
            :data="fetchedData"
            class="my-2 break-inside-avoid-page"
            @highlighted="(n) => (highlightedSerie = n)"
          />
        </div>
      </LoadingOverlay>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { ref, watch, computed } from "vue";
import { useFetch, type AfterFetchContext } from "@vueuse/core";
import { useRouter, useRoute } from "vue-router";
import { ResizeRow } from "vue-resizer";
import LZString from "lz-string";
import InfoBox from "@/components/InfoBox.vue";
import LoadingOverlay from "@/components/LoadingOverlay.vue";
import RequestSummary from "./VisualizePage/RequestSummary.vue";
import DataTable from "./VisualizePage/DataTable.vue";
import DataGraph from "./VisualizePage/DataGraph.vue";
import {
  default as OptionsPanel,
  type ModelType,
} from "./VisualizePage/OptionsPanel.vue";
import type { GraphType } from "./VisualizePage/graphtypes";
import type {
  GraphSankeyHandlerInput,
  GraphLineHandlerInput,
  GraphSankeyHandlerOutput,
  GraphLineHandlerOutput,
  GraphSankeyHandlerResult,
  GraphLineHandlerResult,
} from "./VisualizePage";
import { isEqual, omit, pick } from "lodash-es";

const props = defineProps<{ routeState?: string }>();

const graphHeight = ref(500);
const highlightedSerie = ref<number | null>(null);

const updateTimeRange = ([start, end]: [Date, Date]) => {
  if (state.value === null) return;
  state.value = {
    ...state.value,
    start: start.toISOString(),
    end: end.toISOString(),
    humanStart: start.toISOString(),
    humanEnd: end.toISOString(),
  };
};

// Main state
const state = ref<ModelType>(null);

// Load data from URL
const route = useRoute();
const router = useRouter();
const decodeState = (serialized: string | undefined): ModelType => {
  try {
    if (!serialized) {
      console.debug("no state");
      return null;
    }
    const unserialized = LZString.decompressFromBase64(serialized);
    if (!unserialized) {
      console.debug("empty state");
      return null;
    }
    return JSON.parse(unserialized);
  } catch (error) {
    console.error("cannot decode state:", error);
    return null;
  }
};
const encodeState = (state: ModelType) => {
  if (state === null) return "";
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
const fetchedData = ref<
  GraphLineHandlerResult | GraphSankeyHandlerResult | null
>(null);
const orderedJSONPayload = <T extends Record<string, any>>(input: T): T => {
  return Object.keys(input)
    .sort()
    .reduce(
      (o, k) => ((o[k] = input[k]), o),
      {} as { [key: string]: any }
    ) as T;
};
const jsonPayload = computed(
  (): GraphSankeyHandlerInput | GraphLineHandlerInput | null => {
    if (state.value === null) return null;
    if (state.value.graphType === "sankey") {
      const input: GraphSankeyHandlerInput = {
        ...omit(state.value, [
          "graphType",
          "bidirectional",
          "previousPeriod",
          "humanStart",
          "humanEnd",
        ]),
      };
      return orderedJSONPayload(input);
    } else {
      const input: GraphLineHandlerInput = {
        ...omit(state.value, [
          "graphType",
          "previousPeriod",
          "humanStart",
          "humanEnd",
        ]),
        points: state.value.graphType === "grid" ? 50 : 200,
        "previous-period": state.value.previousPeriod,
      };
      return orderedJSONPayload(input);
    }
  }
);
const request = ref<ModelType>(null); // Same as state, but once request is successful
const { data, execute, isFetching, aborted, abort, canAbort, error } = useFetch(
  "",
  {
    beforeFetch(ctx) {
      // Add the URL. Not a computed value as if we change both payload
      // and URL, the query will be triggered twice.
      const { cancel } = ctx;
      if (state.value === null) {
        cancel();
        return ctx;
      }
      const endpoint: Record<GraphType, string> = {
        stacked: "line",
        stacked100: "line",
        lines: "line",
        grid: "line",
        sankey: "sankey",
      };
      const url = endpoint[state.value.graphType];
      return {
        ...ctx,
        url: `/api/v0/console/graph/${url}`,
      };
    },
    async afterFetch(
      ctx: AfterFetchContext<GraphLineHandlerOutput | GraphSankeyHandlerOutput>
    ) {
      // Update data. Not done in a computed value as we want to keep the
      // previous data in case of errors.
      const { data, response } = ctx;
      if (data === null || !state.value) return ctx;
      console.groupCollapsed("SQL query");
      console.info(
        response.headers.get("x-sql-query")?.replace(/ {2}( )*/g, "\n$1")
      );
      console.groupEnd();
      if (state.value.graphType === "sankey") {
        fetchedData.value = {
          graphType: "sankey",
          ...(data as GraphSankeyHandlerOutput),
          ...pick(state.value, ["start", "end", "dimensions", "units"]),
        };
      } else {
        fetchedData.value = {
          graphType: state.value.graphType,
          ...(data as GraphLineHandlerOutput),
          ...pick(state.value, [
            "start",
            "end",
            "dimensions",
            "units",
            "bidirectional",
          ]),
        };
      }

      // Also update URL.
      const routeTarget = {
        name: "VisualizeWithState",
        params: { state: encodedState.value },
      };
      if (route.name !== "VisualizeWithState") {
        await router.replace(routeTarget);
      } else {
        await router.push(routeTarget);
      }

      // Keep current payload for state
      request.value = state.value;

      return ctx;
    },
    immediate: false,
  }
)
  .post(jsonPayload, "json")
  .json<
    GraphLineHandlerOutput | GraphSankeyHandlerOutput | { message: string }
  >();
watch(jsonPayload, () => execute(), { immediate: true });

const errorMessage = computed(() => {
  if (!error.value || aborted.value) return "";
  if (data.value && "message" in data.value) return data.value.message;
  return `Server returned an error: ${error.value}`;
});
</script>
