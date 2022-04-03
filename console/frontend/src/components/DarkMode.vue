<template>
  <button type="button"
           @click="toggle()"
          class="text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 focus:outline-none focus:ring-4 focus:ring-gray-200 dark:focus:ring-gray-700 rounded-lg text-sm p-2.5">
    <MoonIcon class="w-5 h-5" v-if="!dark" />
    <SunIcon class="w-5 h-5" v-if="dark" />
  </button>
</template>

<script setup>
 import { ref, watch } from 'vue';
 import { SunIcon, MoonIcon } from '@heroicons/vue/solid';

 const dark = ref(false);

 if (localStorage.getItem('color-theme') === 'dark' ||
     (!('color-theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
   dark.value = true;
 }

 watch(dark, (dark) => {
   if (dark) {
     document.documentElement.classList.add('dark');
   } else {
     document.documentElement.classList.remove('dark');
   }
 }, { immediate: true });

 const toggle = () => {
   dark.value = !dark.value;
   localStorage.setItem('color-theme', dark.value?'dark':'light');
 }
</script>
