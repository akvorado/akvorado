<template>
  <slot></slot>
</template>

<script setup>
import { ref, provide, watch, readonly } from "vue";

const isDark = ref(false);
const toggleDark = () => {
  isDark.value = !isDark.value;
  localStorage.setItem("color-theme", isDark.value ? "dark" : "light");
};

if (
  localStorage.getItem("color-theme") === "dark" ||
  (!("color-theme" in localStorage) &&
    window.matchMedia("(prefers-color-scheme: dark)").matches)
) {
  isDark.value = true;
}

watch(
  isDark,
  (isDark) => {
    if (isDark) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  },
  { immediate: true }
);

provide("theme", {
  isDark: readonly(isDark),
  toggleDark,
});
</script>
