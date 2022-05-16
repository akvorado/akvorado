<template>
  <aside
    class="transition-height transition-width relative mb-2 w-full shrink-0 shadow duration-100 lg:mr-4 lg:mb-0 lg:h-auto"
    :class="open ? 'h-96 lg:w-64' : 'h-6 lg:w-6'"
  >
    <button
      class="absolute right-4 bottom-0 z-10 translate-y-1/2 rounded-full bg-white shadow transition-transform delay-100 duration-500 hover:bg-gray-50 dark:bg-gray-900 dark:hover:bg-black lg:top-2 lg:bottom-auto lg:right-0 lg:translate-x-1/2 lg:translate-y-0"
      :class="open && 'rotate-180'"
      @click="open = !open"
    >
      <ChevronRightIcon class="hidden h-8 lg:inline" />
      <ChevronDownIcon class="h-8 lg:hidden" />
    </button>
    <form
      class="h-full overflow-y-auto bg-gray-200 dark:bg-slate-600"
      autocomplete="off"
      spellcheck="false"
      @submit.prevent="apply()"
    >
      <div v-if="open" class="flex h-full flex-col py-4 px-3 lg:max-h-screen">
        <p
          class="my-2 block text-sm font-semibold text-gray-900 dark:text-gray-400"
        >
          Time range
        </p>
        <InputTimeRange v-model="timeRange" />
        <label
          for="dimensions"
          class="my-2 block text-sm font-semibold text-gray-900 dark:text-gray-400"
        >
          Dimensions
        </label>
        <InputDimensions v-model="dimensions" />
        <label
          for="options"
          class="my-2 block text-sm font-semibold text-gray-900 dark:text-gray-400"
        >
          Filter
        </label>
        <textarea
          id="options"
          v-model="filter"
          rows="5"
          class="mb-2 block w-full grow resize-none rounded-lg border border-gray-300 bg-gray-50 p-2.5 font-mono text-sm text-gray-900 focus:border-blue-500 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:placeholder-gray-400 dark:focus:border-blue-500 dark:focus:ring-blue-500"
        ></textarea>
        <div>
          <button
            type="submit"
            :disabled="!!hasErrors"
            :class="
              !!hasErrors && 'cursor-not-allowed bg-blue-400 dark:bg-blue-500'
            "
            class="inline items-center rounded-lg bg-blue-700 px-5 py-2.5 text-center text-sm font-medium text-white hover:bg-blue-800 focus:ring-4 focus:ring-blue-200 dark:focus:ring-blue-900"
          >
            {{ applyLabel }}
          </button>
        </div>
      </div>
    </form>
  </aside>
</template>

<script setup>
import { ref, watch, computed } from "vue";
import { ChevronDownIcon, ChevronRightIcon } from "@heroicons/vue/solid";
import InputTimeRange from "./InputTimeRange.vue";
import InputDimensions from "./InputDimensions.vue";

const props = defineProps({
  state: {
    type: Object,
    default: () => {},
  },
});
const emit = defineEmits(["update"]);

const open = ref(false);

const timeRange = ref({});
const dimensions = ref([]);
const filter = ref("");

const options = computed(() => ({
  points: props.state.points,
  start: timeRange.value.start,
  end: timeRange.value.end,
  dimensions: dimensions.value.selected,
  limit: dimensions.value.limit,
  filter: filter.value,
}));
const applyLabel = computed(() =>
  JSON.stringify(options.value) === JSON.stringify(props.state)
    ? "Refresh"
    : "Apply"
);
const hasErrors = computed(
  () => !!(timeRange.value.errors || dimensions.value.errors)
);

const apply = () => {
  emit("update", options.value);
};

watch(
  () => props.state,
  (state) => {
    const {
      start,
      end,
      dimensions: _dimensions,
      limit,
      points /* eslint-disable-line no-unused-vars */,
      filter: _filter,
    } = JSON.parse(JSON.stringify(state));
    timeRange.value = { start, end };
    dimensions.value = { selected: _dimensions, limit };
    filter.value = _filter;
  },
  { immediate: true, deep: true }
);
</script>
