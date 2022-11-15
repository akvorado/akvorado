<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <slot></slot>
</template>

<script lang="ts" setup>
import { provide, readonly } from "vue";
import { useDark, useToggle } from "@vueuse/core";

const isDark = useDark();
const toggleDark = useToggle(isDark);

provide(ThemeKey, {
  isDark: readonly(isDark),
  toggleDark,
});
</script>

<script lang="ts">
import type { InjectionKey, Ref } from "vue";
export const ThemeKey: InjectionKey<{
  isDark: Readonly<Ref<boolean>>;
  toggleDark: () => void;
}> = Symbol();
</script>
