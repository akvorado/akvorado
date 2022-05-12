<template>
  <aside
    class="relative mr-4 mb-2 w-full shrink-0 shadow lg:h-auto"
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
    <div class="h-full bg-gray-200 dark:bg-gray-700">
      <div v-if="open" class="flex h-full flex-col py-4 px-3 lg:max-h-screen">
        <label
          for="options"
          class="mb-2 block text-sm font-semibold text-gray-900 dark:text-gray-400"
          >Options</label
        >
        <textarea
          id="options"
          v-model="options"
          class="mb-2 block w-full grow rounded-lg border border-gray-300 bg-gray-50 p-2.5 font-mono text-sm text-gray-900 focus:border-blue-500 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:placeholder-gray-400 dark:focus:border-blue-500 dark:focus:ring-blue-500"
        ></textarea>
        <p
          v-if="error"
          class="mb-2 text-xs font-medium text-red-600 dark:text-red-400"
        >
          {{ error }}
        </p>
        <div>
          <button
            type="submit"
            class="inline items-center rounded-lg bg-blue-700 px-5 py-2.5 text-center text-sm font-medium text-white hover:bg-blue-800 focus:ring-4 focus:ring-blue-200 dark:focus:ring-blue-900"
            @click="apply()"
          >
            {{ applyLabel }}
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup>
import { ref, watch, computed } from "vue";
import {
  ChevronLeftIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  ChevronUpIcon,
} from "@heroicons/vue/solid";

import YAML from "yaml";

const props = defineProps({
  state: {
    type: Object,
    default: () => {},
  },
});
const emit = defineEmits(["update"]);

const options = ref("");
const initialOptions = ref("");
const error = ref("");
const open = ref(false);
const applyLabel = computed(() =>
  options.value === initialOptions.value ? "Refresh" : "Apply"
);

const apply = () => {
  error.value = "";
  try {
    emit("update", YAML.parse(options.value));
  } catch (err) {
    console.log(err);
    error.value = `${err}`;
  }
};

watch(
  () => props.state,
  (state) => {
    initialOptions.value = options.value = YAML.stringify(state, null, 1);
  },
  { immediate: true }
);
</script>
