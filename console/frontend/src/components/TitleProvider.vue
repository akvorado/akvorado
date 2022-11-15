<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <slot></slot>
</template>

<script lang="ts" setup>
import { provide, computed, ref } from "vue";
import { useTitle } from "@vueuse/core";
import { useRouter, useRoute } from "vue-router";

// Title has 3 parts:
//  - application name (fixed)
//  - view name (set by router)
//  - document title (set by current view)
const route = useRoute();
const applicationName = "Akvorado";
const viewName = computed(() => route.meta?.title);
const documentTitle = ref<string | null>(null);
const title = computed(() =>
  [applicationName, viewName.value, documentTitle.value]
    .filter((k) => !!k)
    .join(" | ")
);
useTitle(title);
useRouter().beforeEach((to, from) => {
  if (to.meta?.title !== from.meta?.title) {
    documentTitle.value = null;
  }
});

provide(TitleKey, { set: (t: string) => (documentTitle.value = t) });
</script>

<script lang="ts">
import type { InjectionKey } from "vue";
export const TitleKey: InjectionKey<{
  set: (t: string) => void;
}> = Symbol();
</script>
