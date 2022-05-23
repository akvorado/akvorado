<template>
  <aside
    class="transition-height transition-width relative w-full shrink-0 duration-100 lg:h-auto"
    :class="open ? 'h-96 lg:w-64' : 'h-6 lg:w-6'"
  >
    <button
      class="absolute right-4 bottom-0 z-20 translate-y-1/2 rounded-full bg-white shadow transition-transform delay-100 duration-500 hover:bg-gray-300 dark:bg-gray-900 dark:hover:bg-black lg:top-2 lg:bottom-auto lg:right-0 lg:translate-x-1/2 lg:translate-y-0"
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
          <template #item="{ name }">{{ name }}</template>
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
        <div class="flex flex-col items-end">
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
defineEmits(["update:modelValue", "cancel"]);

import { ref, watch, computed } from "vue";
import { ChevronDownIcon, ChevronRightIcon } from "@heroicons/vue/solid";
import InputTimeRange from "@/components/InputTimeRange.vue";
import InputDimensions from "@/components/InputDimensions.vue";
import InputTextarea from "@/components/InputTextarea.vue";
import InputListBox from "@/components/InputListBox.vue";
import InputButton from "@/components/InputButton.vue";
import SectionLabel from "./SectionLabel.vue";
import { graphTypes } from "./constants";
import isEqual from "lodash.isequal";

const graphTypeList = Object.entries(graphTypes).map(([, v], idx) => ({
  id: idx + 1,
  name: v,
}));

const open = ref(false);
const graphType = ref(graphTypeList[0]);
const timeRange = ref({});
const dimensions = ref([]);
const filter = ref("");

const options = computed(() => ({
  points: graphType.value.name === graphTypes.multigraph ? 50 : 200,
  start: timeRange.value.start,
  end: timeRange.value.end,
  dimensions: dimensions.value.selected,
  limit: dimensions.value.limit,
  filter: filter.value,
  graphType: graphType.value.name,
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
      graphType: _graphType,
      start,
      end,
      dimensions: _dimensions,
      limit,
      points /* eslint-disable-line no-unused-vars */,
      filter: _filter,
    } = JSON.parse(JSON.stringify(modelValue));
    graphType.value =
      graphTypeList.find(({ name }) => name === _graphType) || graphTypeList[0];
    timeRange.value = { start, end };
    dimensions.value = { selected: _dimensions, limit };
    filter.value = _filter;
  },
  { immediate: true, deep: true }
);
</script>
