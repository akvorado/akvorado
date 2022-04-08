<template>
  <button
    type="button"
    class="rounded-lg p-2.5 text-sm text-gray-500 hover:bg-gray-100 focus:outline-none focus:ring-4 focus:ring-gray-200 dark:text-gray-400 dark:hover:bg-gray-700 dark:focus:ring-gray-700"
    @click="toggle()"
  >
    <MoonIcon v-if="!dark" class="h-5 w-5" />
    <SunIcon v-if="dark" class="h-5 w-5" />
  </button>
</template>

<script setup>
import { ref, watch } from "vue";
import { SunIcon, MoonIcon } from "@heroicons/vue/solid";

const dark = ref(false);

if (
  localStorage.getItem("color-theme") === "dark" ||
  (!("color-theme" in localStorage) &&
    window.matchMedia("(prefers-color-scheme: dark)").matches)
) {
  dark.value = true;
}

watch(
  dark,
  (dark) => {
    if (dark) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  },
  { immediate: true }
);

const toggle = () => {
  dark.value = !dark.value;
  localStorage.setItem("color-theme", dark.value ? "dark" : "light");
};
</script>
