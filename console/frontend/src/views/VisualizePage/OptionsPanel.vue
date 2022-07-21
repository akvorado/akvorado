<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <aside
    class="transition-height transition-width w-full shrink-0 duration-100 lg:h-auto"
    :class="open ? 'h-80 lg:w-72' : 'h-4 lg:w-4'"
  >
    <span
      class="absolute z-40 translate-x-4 transition-transform lg:translate-y-4"
      :class="
        open
          ? 'translate-y-80 rotate-180 lg:translate-x-72'
          : 'translate-y-4 lg:translate-x-0'
      "
    >
      <button
        class="flex h-4 w-4 items-center justify-center rounded-full bg-white shadow transition-transform duration-100 hover:bg-gray-300 dark:bg-gray-900 dark:shadow-white/10 dark:hover:bg-black lg:translate-x-1/2 lg:translate-y-0"
        :class="open ? 'translate-y-1/2' : '-translate-y-1/2'"
        @click="open = !open"
      >
        <ChevronRightIcon class="hidden lg:inline" />
        <ChevronDownIcon class="lg:hidden" />
      </button>
    </span>
    <form
      class="h-full overflow-y-auto border-b border-gray-300 bg-gray-100 dark:border-slate-700 dark:bg-slate-800 lg:border-r lg:border-b-0"
      autocomplete="off"
      spellcheck="false"
      @submit.prevent="
        loading ? $emit('cancel') : $emit('update:modelValue', options)
      "
    >
      <div v-if="open" class="flex flex-col px-3 py-4 lg:max-h-screen">
        <div
          class="mb-2 flex flex-row flex-wrap justify-between gap-2 sm:flex-nowrap lg:flex-wrap"
        >
          <InputButton
            attr-type="submit"
            :disabled="hasErrors && !loading"
            :loading="loading"
            :type="loading ? 'alternative' : 'primary'"
            class="order-2 w-28 justify-center sm:order-3 lg:order-2"
          >
            {{ loading ? "Cancel" : applyLabel }}
          </InputButton>
          <InputChoice
            v-model="units"
            :choices="[
              { label: 'L3ᵇ⁄ₛ', name: 'l3bps' },
              { label: 'L2ᵇ⁄ₛ', name: 'l2bps' },
              { label: 'ᵖ⁄ₛ', name: 'pps' },
            ]"
            label="Unit"
            class="order-1"
          />
          <InputListBox
            v-model="graphType"
            :items="graphTypeList"
            class="order-3 grow basis-full sm:order-2 sm:basis-0 lg:order-3"
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
        </div>
        <SectionLabel>Time range</SectionLabel>
        <InputTimeRange v-model="timeRange" />
        <SectionLabel>Dimensions</SectionLabel>
        <InputDimensions
          v-model="dimensions"
          :min-dimensions="graphType.name === graphTypes.sankey ? 2 : 0"
        />
        <SectionLabel>
          <template #default>Filter</template>
          <template #hint>
            <kbd
              class="rounded border border-gray-300 bg-gray-200 px-1 dark:border-gray-600 dark:bg-gray-900"
              >Ctrl-Space</kbd
            >
            for completions
          </template>
        </SectionLabel>
        <InputFilter v-model="filter" class="mb-2" />
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

import { ref, watch, computed, inject } from "vue";
import { ChevronDownIcon, ChevronRightIcon } from "@heroicons/vue/solid";
import InputTimeRange from "@/components/InputTimeRange.vue";
import InputDimensions from "@/components/InputDimensions.vue";
import InputListBox from "@/components/InputListBox.vue";
import InputButton from "@/components/InputButton.vue";
import InputChoice from "@/components/InputChoice.vue";
import InputFilter from "@/components/InputFilter.vue";
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
const filter = ref({});
const units = ref("l3bps");

const options = computed(() => ({
  // Common to all graph types
  graphType: graphType.value.name,
  start: timeRange.value.start,
  end: timeRange.value.end,
  dimensions: dimensions.value.selected,
  limit: dimensions.value.limit,
  filter: filter.value.expression,
  units: units.value,
  // Only for time series
  ...([stacked, lines].includes(graphType.value.name) && { points: 200 }),
  ...(graphType.value.name === grid && { points: 50 }),
}));
const applyLabel = computed(() =>
  isEqual(options.value, props.modelValue) ? "Refresh" : "Apply"
);
const hasErrors = computed(
  () =>
    !!(timeRange.value.errors || dimensions.value.errors || filter.value.errors)
);

const serverConfiguration = inject("server-configuration");
watch(
  () => [props.modelValue, serverConfiguration.value?.defaultVisualizeOptions],
  ([modelValue, defaultOptions]) => {
    if (defaultOptions === undefined) return;
    const {
      graphType: _graphType = graphTypes.stacked,
      start = defaultOptions?.start,
      end = defaultOptions?.end,
      dimensions: _dimensions = defaultOptions?.dimensions,
      limit = 10,
      points /* eslint-disable-line no-unused-vars */,
      filter: _filter = defaultOptions?.filter,
      units: _units = "l3bps",
    } = modelValue;

    // Dispatch values in refs
    graphType.value =
      graphTypeList.find(({ name }) => name === _graphType) || graphTypeList[0];
    timeRange.value = { start, end };
    dimensions.value = {
      selected: [..._dimensions],
      limit,
    };
    filter.value = { expression: _filter };
    units.value = _units;

    // A bit risky, but it seems to work.
    if (!isEqual(modelValue, options.value)) {
      open.value = true;
      if (!hasErrors.value && start) {
        emit("update:modelValue", options.value);
      }
    }
  },
  { immediate: true, deep: true }
);
</script>
