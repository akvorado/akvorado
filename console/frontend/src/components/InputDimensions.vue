<template>
  <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-1">
    <Listbox
      v-model="selectedDimensions"
      multiple
      class="col-span-2 lg:col-span-1"
    >
      <div class="relative">
        <ListboxButton
          id="dimensions"
          class="peer relative block w-full appearance-none rounded-t-lg border-0 border-b-2 border-gray-300 bg-gray-50 pl-2.5 pr-8 pb-1.5 pt-4 text-left text-sm text-gray-900 focus:border-blue-600 focus:outline-none focus:ring-0 dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:focus:border-blue-500"
        >
          <span class="block flex flex-wrap gap-2">
            <span v-if="selectedDimensions.length === 0">No dimensions</span>
            <span
              v-for="dimension in selectedDimensions"
              :key="dimension.id"
              class="-mb-[4px] flex items-center gap-1 rounded border-2 bg-violet-100 px-2 dark:bg-slate-800"
              :style="{ borderColor: dimension.color }"
            >
              <span>{{ dimension.name }}</span>
              <XIcon
                class="h-4 w-4 cursor-pointer"
                @click.stop.prevent="removeDimension(dimension)"
              />
            </span>
          </span>
          <span
            class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2"
          >
            <SelectorIcon class="h-5 w-5 text-gray-400" aria-hidden="true" />
          </span>
        </ListboxButton>
        <label
          for="dimensions"
          class="z-5 absolute top-3 left-2.5 origin-[0] -translate-y-3 scale-75 transform text-sm text-gray-500 peer-focus:text-blue-600 dark:text-gray-400 dark:peer-focus:text-blue-500"
        >
          Dimensions
        </label>

        <transition
          leave-active-class="transition duration-100 ease-in"
          leave-from-class="opacity-100"
          leave-to-class="opacity-0"
          class="z-10 rounded bg-white shadow dark:bg-gray-700"
        >
          <ListboxOptions
            class="absolute max-h-60 w-full overflow-auto py-1 text-sm text-gray-700 dark:text-gray-200"
          >
            <ListboxOption
              v-for="dimension in dimensions"
              v-slot="{ active, selected }"
              :key="dimension.id"
              :value="dimension"
              as="template"
            >
              <li
                class="relative inline-flex w-full cursor-default select-none py-2 pl-10 pr-4 text-sm hover:bg-gray-100 dark:hover:bg-gray-600 dark:hover:text-white"
                :class="
                  active &&
                  'bg-gray-100 dark:bg-gray-600 dark:bg-gray-600 dark:text-white'
                "
              >
                <span class="block truncate">
                  <span
                    :style="{ backgroundColor: dimension.color }"
                    class="inline w-1 rounded"
                    >&nbsp;</span
                  >
                  {{ dimension.name }}
                </span>
                <span
                  v-if="selected"
                  class="absolute inset-y-0 left-0 flex items-center pl-3 text-blue-600 dark:text-blue-500"
                >
                  <CheckIcon class="h-5 w-5" aria-hidden="true" />
                </span>
              </li>
            </ListboxOption>
          </ListboxOptions>
        </transition>
      </div>
    </Listbox>
    <InputString id="limit" v-model="limit" label="Limit" :error="limitError" />
  </div>
</template>

<script setup>
import { ref, watch, computed } from "vue";
import {
  Listbox,
  ListboxButton,
  ListboxOptions,
  ListboxOption,
} from "@headlessui/vue";
import { XIcon, CheckIcon, SelectorIcon } from "@heroicons/vue/solid";
import { dataColor } from "@/utils";
import InputString from "@/components/InputString.vue";
import fields from "@data/fields.json";

const props = defineProps({
  modelValue: {
    // selected: selected dimensions (names)
    // limit: limit as an integer
    // errors: is there an input error?
    type: Object,
    required: true,
  },
});
const emit = defineEmits(["update:modelValue"]);

const selectedDimensions = ref([]);
const limit = ref("10");
const limitError = computed(() => {
  const val = parseInt(limit.value);
  if (isNaN(val)) {
    return "Not a number";
  }
  if (val < 5) {
    return "Should be more than 5";
  }
  if (val > 50) {
    return "Should be less than 50";
  }
  return "";
});

const dimensions = fields.map((v, idx) => ({
  id: idx + 1,
  name: v,
  color: dataColor(
    ["Exporter", "Src", "Dst", "In", "Out", ""]
      .map((p) => v.startsWith(p))
      .indexOf(true)
  ),
}));

const removeDimension = (dimension) => {
  selectedDimensions.value = selectedDimensions.value.filter(
    (d) => d !== dimension
  );
};

watch(
  () => props.modelValue,
  (model) => {
    limit.value = model.limit.toString();
    selectedDimensions.value = model.selected
      .map((name) => dimensions.filter((d) => d.name === name)[0])
      .filter((d) => d !== undefined);
  },
  { immediate: true, deep: true }
);
watch(
  [selectedDimensions, limit, limitError],
  ([selected, limit, limitError]) => {
    const updated = {
      selected: selected.map((d) => d.name),
      limit: parseInt(limit) || limit,
      errors: !!limitError,
    };
    if (JSON.stringify(updated) !== JSON.stringify(props.modelValue)) {
      emit("update:modelValue", updated);
    }
  }
);
</script>
