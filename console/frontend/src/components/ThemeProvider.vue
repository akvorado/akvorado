<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

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
