<template>
  <InputComponent v-slot="{ id, childClass }">
    <textarea
      :id="id"
      ref="el"
      :style="{ height: height }"
      :class="childClass"
      class="resize-none appearance-none"
      placeholder=" "
      v-bind="$attrs"
      :value="modelValue"
      @input="$emit('update:modelValue', $event.target.value)"
      @focus="resize"
    />
  </InputComponent>
</template>

<script setup>
const props = defineProps({
  maxHeight: {
    type: [Number],
    default: null,
  },
  autosize: {
    type: Boolean,
    default: false,
  },
  modelValue: {
    type: String,
    required: true,
  },
});
defineEmits(["update:modelValue"]);

import { ref, watch, nextTick, onMounted, onBeforeUnmount } from "vue";
import InputComponent from "@/components/InputComponent.vue";

const el = ref(null);
const height = ref("auto");

const resize = async () => {
  if (el.value === null) {
    return;
  }
  height.value = "auto";
  await nextTick();
  const {
    borderTopWidth: styleBorderTop,
    borderBottomWidth: styleBorderBottom,
  } = window.getComputedStyle(el.value);
  let contentHeight = el.value.scrollHeight;
  contentHeight += Number(styleBorderTop.slice(0, -2));
  contentHeight += Number(styleBorderBottom.slice(0, -2));
  if (props.maxHeight && contentHeight > props.maxHeight) {
    contentHeight = props.maxHeight;
  }
  height.value = contentHeight + "px";
  return true;
};

const width = ref(null);
const observer = new ResizeObserver((entries) => {
  for (let entry of entries) {
    if (entry.contentRect.width !== width.value) {
      width.value = entry.contentRect.width;
    }
  }
});

watch(() => [props.maxHeight, props.modelValue, width.value], resize);
onMounted(() => {
  observer.observe(el.value);
  resize();
});
onBeforeUnmount(() => {
  observer.disconnect();
});
</script>
