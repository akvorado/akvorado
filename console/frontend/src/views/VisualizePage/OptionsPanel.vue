<template>
  <aside
    class="transition-height transition-width w-full shrink-0 duration-100 lg:h-auto"
    :class="open ? 'h-80 lg:w-72' : 'h-4 lg:w-4'"
  >
    <span
      class="absolute z-30 translate-x-4 transition-transform lg:translate-y-4"
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
      <div v-if="open" class="flex h-full flex-col py-4 px-3 lg:max-h-screen">
        <div class="mb-2 flex flex-row flex-wrap justify-between gap-2">
          <InputButton
            attr-type="submit"
            :disabled="hasErrors && !loading"
            :loading="loading"
            :type="loading ? 'default' : 'primary'"
            class="order-3 w-28 justify-center lg:order-2 lg:grow-0"
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
            class="order-2 lg:order-1 lg:grow-0"
          />
          <InputListBox
            v-model="graphType"
            :items="graphTypeList"
            class="order-1 grow lg:order-3 lg:h-full"
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
        <div
          class="lg:focus-within:pointer-events-none lg:focus-within:absolute lg:focus-within:inset-0 lg:focus-within:z-40 lg:focus-within:bg-black/30"
        >
          <div
            class="lg:focus-within:pointer-events-auto lg:focus-within:absolute lg:focus-within:inset-x-2 lg:focus-within:top-36 lg:focus-within:z-50 lg:focus-within:rounded lg:focus-within:bg-gray-100 lg:focus-within:p-3"
          >
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
import InputFilter from "@/components/InputFilter.vue";
import InputListBox from "@/components/InputListBox.vue";
import InputButton from "@/components/InputButton.vue";
import InputChoice from "@/components/InputChoice.vue";
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
      units: _units = "l3bps",
    } = modelValue;

    // Dispatch values in refs
    graphType.value =
      graphTypeList.find(({ name }) => name === _graphType) || graphTypeList[0];
    timeRange.value = { start, end };
    dimensions.value = { selected: [..._dimensions], limit };
    filter.value = { expression: _filter };
    units.value = _units;

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
