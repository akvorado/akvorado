<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <button
    type="button"
    class="rounded-lg p-2.5 text-sm text-gray-500 hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-300 dark:text-gray-400 dark:hover:bg-gray-700 dark:focus:ring-blue-800"
    @click="toggleDark()"
  >
    <MoonIcon v-if="!isDark" class="h-5 w-5" />
    <SunIcon v-if="isDark" class="h-5 w-5" />
  </button>
</template>

<script setup>
import { inject } from "vue";
import { SunIcon, MoonIcon } from "@heroicons/vue/solid";
const { isDark, toggleDark } = inject("theme");
</script>
