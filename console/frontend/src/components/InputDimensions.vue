<template>
  <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-1">
    <InputListBox
      v-model="selectedDimensions"
      :items="dimensions"
      :error="dimensionsError"
      multiple
      label="Dimensions"
      class="col-span-2 lg:col-span-1"
    >
      <template #selected>
        <span v-if="selectedDimensions.length === 0">No dimensions</span>
        <draggable
          v-model="selectedDimensions"
          class="block flex flex-wrap gap-1"
          tag="span"
          item-key="id"
        >
          <template #item="{ element: dimension }">
            <span
              class="flex items-center gap-1 rounded border-2 bg-violet-100 px-1.5 dark:bg-slate-800 dark:text-gray-200"
              :style="{ borderColor: dimension.color }"
            >
              <span class="leading-4">{{ dimension.name }}</span>
              <XIcon
                class="h-4 w-4 cursor-pointer"
                @click.stop.prevent="removeDimension(dimension)"
              />
            </span>
          </template>
        </draggable>
        <span
          class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2"
        >
          <SelectorIcon class="h-5 w-5 text-gray-400" aria-hidden="true" />
        </span>
      </template>
      <template #item="{ name, color }">
        <span :style="{ backgroundColor: color }" class="inline w-1 rounded"
          >&nbsp;</span
        >
        {{ name }}
      </template>
    </InputListBox>
    <InputString v-model="limit" label="Limit" :error="limitError" />
  </div>
</template>

<script setup>
const props = defineProps({
  modelValue: {
    // selected: selected dimensions (names)
    // limit: limit as an integer
    // errors: is there an input error?
    type: Object,
    required: true,
  },
  minDimensions: {
    type: Number,
    default: 0,
  },
});
const emit = defineEmits(["update:modelValue"]);

import { ref, watch, computed } from "vue";
import draggable from "vuedraggable";
import { XIcon, SelectorIcon } from "@heroicons/vue/solid";
import { dataColor } from "@/utils";
import InputString from "@/components/InputString.vue";
import InputListBox from "@/components/InputListBox.vue";
import fields from "@data/fields.json";
import { isEqual } from "lodash-es";

const selectedDimensions = ref([]);
const dimensionsError = computed(() => {
  if (selectedDimensions.value.length < props.minDimensions) {
    return "At least two dimensions are required";
  }
  return "";
});
const limit = ref("10");
const limitError = computed(() => {
  const val = parseInt(limit.value);
  if (isNaN(val)) {
    return "Not a number";
  }
  if (val < 1) {
    return "Should be ≥ 1";
  }
  if (val > 50) {
    return "Should be ≤ 50";
  }
  return "";
});
const hasErrors = computed(() => !!limitError.value || !!dimensionsError.value);

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
  (value) => {
    limit.value = value.limit.toString();
    selectedDimensions.value = value.selected
      .map((name) => dimensions.find((d) => d.name === name))
      .filter((d) => d !== undefined);
  },
  { immediate: true, deep: true }
);
watch(
  [selectedDimensions, limit, hasErrors],
  ([selected, limit, hasErrors]) => {
    const updated = {
      selected: selected.map((d) => d.name),
      limit: parseInt(limit) || limit,
      errors: hasErrors,
    };
    if (!isEqual(updated, props.modelValue)) {
      emit("update:modelValue", updated);
    }
  }
);
</script>
