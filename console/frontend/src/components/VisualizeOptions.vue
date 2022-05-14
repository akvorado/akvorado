<template>
  <aside
    class="transition-height transition-width relative mb-2 w-full shrink-0 shadow duration-100 lg:mr-4 lg:mb-0 lg:h-auto"
    :class="open ? 'h-64 lg:w-64' : 'h-6 lg:w-6'"
  >
    <button
      class="absolute right-4 bottom-0 translate-y-1/2 rounded-full bg-white shadow hover:bg-gray-50 dark:bg-gray-900 dark:hover:bg-black lg:top-2 lg:bottom-auto lg:right-0 lg:translate-x-1/2 lg:translate-y-0"
      @click="open = !open"
    >
      <ChevronRightIcon v-if="!open" class="hidden h-8 lg:inline" />
      <ChevronLeftIcon v-if="open" class="hidden h-8 lg:inline" />
      <ChevronDownIcon v-if="!open" class="h-8 lg:hidden" />
      <ChevronUpIcon v-if="open" class="h-8 lg:hidden" />
    </button>
    <form
      class="h-full overflow-y-auto bg-gray-200 dark:bg-gray-700"
      @submit.prevent="apply()"
    >
      <div v-if="open" class="flex h-full flex-col py-4 px-3 lg:max-h-screen">
        <p
          class="my-2 block text-sm font-semibold text-gray-900 dark:text-gray-400"
        >
          Time range
        </p>
        <div class="flex flex-row lg:flex-col">
          <InputFloatingLabel
            id="start"
            v-model="startTime"
            class="mr-1 basis-1/2 lg:mr-0"
            label="Start"
            :error="startTimeError"
          />
          <InputFloatingLabel
            id="end"
            v-model="endTime"
            class="ml-1 basis-1/2 lg:ml-0"
            label="End"
            :error="endTimeError"
          />
        </div>
        <label
          for="options"
          class="my-2 block text-sm font-semibold text-gray-900 dark:text-gray-400"
        >
          Other options
        </label>
        <textarea
          id="options"
          v-model="yamlOptions"
          rows="5"
          class="mb-2 block w-full grow resize-none rounded-lg border border-gray-300 bg-gray-50 p-2.5 font-mono text-sm text-gray-900 focus:border-blue-500 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:placeholder-gray-400 dark:focus:border-blue-500 dark:focus:ring-blue-500"
        ></textarea>
        <p
          v-if="yamlError"
          class="mb-2 text-xs font-medium text-red-600 dark:text-red-400"
        >
          {{ yamlError }}
        </p>
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
import { Date as SugarDate } from "sugar-date";
import {
  ChevronLeftIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  ChevronUpIcon,
} from "@heroicons/vue/solid";
import InputFloatingLabel from "./InputFloatingLabel.vue";

import YAML from "yaml";

const props = defineProps({
  state: {
    type: Object,
    default: () => {},
  },
});
const emit = defineEmits(["update"]);

const open = ref(false);

// Time
const startTime = ref("");
const endTime = ref("");
const parsedStartTime = computed(() => SugarDate.create(startTime.value));
const parsedEndTime = computed(() => SugarDate.create(endTime.value));
const startTimeError = computed(() =>
  isNaN(parsedStartTime.value) ? "Invalid date" : ""
);
const endTimeError = computed(
  () =>
    (isNaN(parsedEndTime.value) ? "Invalid date" : "") ||
    (!isNaN(parsedStartTime.value) &&
      parsedStartTime.value > parsedEndTime.value &&
      "End date should be before start date") ||
    ""
);

// Other options as YAML
const yamlOptions = ref("");
const yamlError = ref("");
watch(yamlOptions, () => {
  yamlError.value = "";
  try {
    YAML.parse(yamlOptions.value);
  } catch (err) {
    yamlError.value = `${err}`;
  }
});

const options = computed(() => {
  try {
    let options = YAML.parse(yamlOptions.value);
    options.start = startTime.value;
    options.end = endTime.value;
    return options;
  } catch (_) {
    return {};
  }
});
const applyLabel = computed(() =>
  JSON.stringify(options.value) === JSON.stringify(props.state)
    ? "Refresh"
    : "Apply"
);
const hasErrors = computed(
  () => !!(yamlError.value || startTimeError.value || endTimeError.value)
);

const apply = () => {
  emit("update", options.value);
};

watch(
  () => props.state,
  (state) => {
    const { start, end, ...otherOptions } = JSON.parse(JSON.stringify(state));
    yamlOptions.value = YAML.stringify(otherOptions);
    startTime.value = start;
    endTime.value = end;
  },
  { immediate: true, deep: true }
);
</script>
