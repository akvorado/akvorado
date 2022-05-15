<template>
  <div>
    <div class="relative">
      <input
        :id="props.id"
        :value="modelValue"
        type="text"
        :class="{
          'dark:focus-border-red-500 border-red-600 focus:border-red-600':
            error,
          'border-gray-300 focus:border-blue-600 dark:border-gray-600 dark:focus:border-blue-500':
            !error,
        }"
        class="peer block w-full appearance-none rounded-t-lg border-0 border-b-2 bg-gray-50 px-2.5 pb-1.5 pt-4 text-sm text-gray-900 focus:outline-none focus:ring-0 dark:bg-gray-700 dark:text-white"
        placeholder=" "
        @input="$emit('update:modelValue', $event.target.value)"
      />
      <label
        :for="props.id"
        :class="{
          'text-red-600 dark:text-red-500': error,
          'text-gray-500 peer-focus:text-blue-600 dark:text-gray-400 dark:peer-focus:text-blue-500':
            !error,
        }"
        class="z-5 absolute top-3 left-2.5 origin-[0] -translate-y-3 scale-75 transform text-sm duration-300 peer-placeholder-shown:translate-y-0 peer-placeholder-shown:scale-100 peer-focus:-translate-y-3 peer-focus:scale-75"
        >{{ props.label }}</label
      >
    </div>
    <p class="text-xs text-red-600 dark:text-red-400">
      {{ props.error }}
    </p>
  </div>
</template>

<script setup>
const props = defineProps({
  label: {
    type: String,
    required: true,
  },
  id: {
    type: String,
    required: true,
  },
  error: {
    type: String,
    default: "",
  },
  modelValue: {
    type: String,
    required: true,
  },
});

defineEmits(["update:modelValue"]);
</script>
