<template>
  <aside
    class="transition-height transition-width relative w-full shrink-0 duration-100 lg:h-auto"
    :class="open ? 'h-96 lg:w-64' : 'h-6 lg:w-6'"
  >
    <button
      class="absolute right-4 bottom-0 z-50 translate-y-1/2 rounded-full bg-white shadow transition-transform delay-100 duration-500 hover:bg-gray-300 dark:bg-gray-900 dark:shadow-white/10 dark:hover:bg-black lg:top-2 lg:bottom-auto lg:right-0 lg:translate-x-1/2 lg:translate-y-0"
      :class="open && 'rotate-180'"
      @click="open = !open"
    >
      <ChevronRightIcon class="hidden h-8 lg:inline" />
      <ChevronDownIcon class="h-8 lg:hidden" />
    </button>
    <form
      class="h-full overflow-y-auto border-r border-gray-300 bg-gray-100 dark:border-slate-700 dark:bg-slate-800"
      autocomplete="off"
      spellcheck="false"
      @submit.prevent="
        loading ? $emit('cancel') : $emit('update:modelValue', options)
      "
    >
      <div v-if="open" class="flex h-full flex-col py-4 px-3 lg:max-h-screen">
        <InputListBox
          v-model="graphType"
          :items="graphTypeList"
          label="Graph type"
        >
          <template #selected>{{ graphType.name }}</template>
          <template #item="{ name }">
            <div class="flex w-full items-center justify-between">
              <span>{{ name }}</span>
              <GraphIcon
                :name="name"
                class="mr-1 inline h-4 text-gray-500 dark:text-gray-400"
              />
            </div>
          </template>
        </InputListBox>

        <SectionLabel>Time range</SectionLabel>
        <InputTimeRange v-model="timeRange" />
        <SectionLabel>Dimensions</SectionLabel>
        <InputDimensions
          v-model="dimensions"
          :min-dimensions="graphType.name === graphTypes.sankey ? 2 : 0"
        />
        <SectionLabel>Filter</SectionLabel>
        <InputTextarea
          v-model="filter"
          rows="1"
          label="Filter expression"
          class="mb-2 font-mono"
          autosize
        />
        <div class="flex flex-row items-start justify-between">
          <InputToggle
            v-model="pps"
            :label="'Unit: ' + (pps ? 'ᵖ⁄ₛ' : 'ᵇ⁄ₛ')"
          />
          <InputButton
            attr-type="submit"
            :disabled="hasErrors && !loading"
            :loading="loading"
            :type="loading ? 'default' : 'primary'"
            class="mb-2"
          >
            {{ loading ? "Cancel" : applyLabel }}
          </InputButton>
        </div>
      </div>
    </form>
  </aside>
</template>

<script setup>
const props = defineProps({
  modelValue: {
    type: Object,
    required: true,
  },
  loading: {
    type: Boolean,
    default: false,
  },
});
const emit = defineEmits(["update:modelValue", "cancel"]);

import { ref, watch, computed } from "vue";
import { ChevronDownIcon, ChevronRightIcon } from "@heroicons/vue/solid";
import InputTimeRange from "@/components/InputTimeRange.vue";
import InputDimensions from "@/components/InputDimensions.vue";
import InputTextarea from "@/components/InputTextarea.vue";
import InputListBox from "@/components/InputListBox.vue";
import InputButton from "@/components/InputButton.vue";
import InputToggle from "@/components/InputToggle.vue";
import SectionLabel from "./SectionLabel.vue";
import GraphIcon from "./GraphIcon.vue";
import { graphTypes } from "./constants";
import { isEqual } from "lodash-es";

const graphTypeList = Object.entries(graphTypes).map(([, v], idx) => ({
  id: idx + 1,
  name: v,
}));
const { stacked, lines, grid } = graphTypes;

const open = ref(false);
const graphType = ref(graphTypeList[0]);
const timeRange = ref({});
const dimensions = ref([]);
const filter = ref("");
const pps = ref(false);

const options = computed(() => ({
  // Common to all graph types
  graphType: graphType.value.name,
  start: timeRange.value.start,
  end: timeRange.value.end,
  dimensions: dimensions.value.selected,
  limit: dimensions.value.limit,
  filter: filter.value,
  units: pps.value ? "pps" : "bps",
  // Only for time series
  ...([stacked, lines].includes(graphType.value.name) && { points: 200 }),
  ...(graphType.value.name === grid && { points: 50 }),
}));
const applyLabel = computed(() =>
  isEqual(options.value, props.modelValue) ? "Refresh" : "Apply"
);
const hasErrors = computed(
  () => !!(timeRange.value.errors || dimensions.value.errors)
);

watch(
  () => props.modelValue,
  (modelValue) => {
    const {
      graphType: _graphType = graphTypes.stacked,
      start = "6 hours ago",
      end = "now",
      dimensions: _dimensions = ["SrcAS", "ExporterName"],
      limit = 10,
      points /* eslint-disable-line no-unused-vars */,
      filter: _filter = "InIfBoundary = external",
      units = "bps",
    } = modelValue;

    // Dispatch values in refs
    graphType.value =
      graphTypeList.find(({ name }) => name === _graphType) || graphTypeList[0];
    timeRange.value = { start, end };
    dimensions.value = { selected: [..._dimensions], limit };
    filter.value = _filter;
    pps.value = units == "pps";

    // A bit risky, but it seems to work.
    if (!isEqual(modelValue, options.value)) {
      open.value = true;
      if (!hasErrors.value) {
        emit("update:modelValue", options.value);
      }
    }
  },
  { immediate: true, deep: true }
);
</script>
