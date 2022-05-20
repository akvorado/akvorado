<template>
  <slot></slot>
</template>

<script setup>
import { provide, readonly } from "vue";
import { useDark, useToggle } from "@vueuse/core";

const isDark = useDark();
const toggleDark = useToggle(isDark);

provide("theme", {
  isDark: readonly(isDark),
  toggleDark,
});
</script>
