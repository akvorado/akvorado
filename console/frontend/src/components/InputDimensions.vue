<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-1">
    <InputListBox
      v-model="selectedDimensions"
      :items="dimensions"
      :error="dimensionsError"
      multiple
      label="Dimensions"
      filter="name"
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
              class="flex cursor-grab items-center gap-1 rounded border-2 bg-violet-100 px-1.5 dark:bg-slate-800 dark:text-gray-200"
              :style="{ borderColor: dimension.color }"
            >
              <span class="leading-4">{{ dimension.name }}</span>
              <XIcon
                class="h-4 w-4 cursor-pointer hover:text-blue-700 dark:hover:text-white"
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

<script lang="ts" setup>
import { ref, watch, computed, inject } from "vue";
import draggable from "vuedraggable";
import { XIcon, SelectorIcon } from "@heroicons/vue/solid";
import { dataColor } from "@/utils";
import InputString from "@/components/InputString.vue";
import InputListBox from "@/components/InputListBox.vue";
import { ServerConfigKey } from "@/components/ServerConfigProvider.vue";
import { isEqual } from "lodash-es";

const props = withDefaults(
  defineProps<{
    modelValue: ModelType;
    minDimensions?: number;
  }>(),
  {
    minDimensions: 0,
  }
);
const emit = defineEmits<{
  (e: "update:modelValue", value: typeof props.modelValue): void;
}>();

const serverConfiguration = inject(ServerConfigKey)!;
const selectedDimensions = ref<Array<typeof dimensions.value[0]>>([]);
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
  const upperLimit = serverConfiguration.value?.dimensionsLimit ?? 50;
  if (val > upperLimit) {
    return `Should be ≤ ${upperLimit}`;
  }
  return "";
});
const hasErrors = computed(() => !!limitError.value || !!dimensionsError.value);

const dimensions = computed(() =>
  serverConfiguration.value?.dimensions.map((v, idx) => ({
    id: idx + 1,
    name: v,
    color: dataColor(
      ["Exporter", "Src", "Dst", "In", "Out", ""]
        .map((p) => v.startsWith(p))
        .indexOf(true)
    ),
  }))
);

const removeDimension = (dimension: typeof dimensions.value[0]) => {
  selectedDimensions.value = selectedDimensions.value.filter(
    (d) => d !== dimension
  );
};

watch(
  () => [props.modelValue, dimensions.value] as const,
  ([value, dimensions]) => {
    if (value) {
      limit.value = value.limit.toString();
    }
    if (value)
      selectedDimensions.value = value.selected
        .map((name) => dimensions.find((d) => d.name === name))
        .filter((d): d is typeof dimensions[0] => !!d);
  },
  { immediate: true, deep: true }
);
watch(
  [selectedDimensions, limit, hasErrors] as const,
  ([selected, limit, hasErrors]) => {
    const updated = {
      selected: selected.map((d) => d.name),
      limit: parseInt(limit) || 10,
      errors: hasErrors,
    };
    if (!isEqual(updated, props.modelValue)) {
      emit("update:modelValue", updated);
    }
  }
);
</script>

<script lang="ts">
export type ModelType = {
  selected: string[];
  limit: number;
  errors?: boolean;
} | null;
</script>
