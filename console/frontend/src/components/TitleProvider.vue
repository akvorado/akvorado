<template>
  <slot></slot>
</template>

<script setup>
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
const documentTitle = ref(null);
const title = computed(() =>
  [applicationName, viewName.value, documentTitle.value]
    .filter((k) => !!k)
    .join(" | ")
);
useTitle(title);
useRouter().afterEach(() => {
  documentTitle.value = null;
});

provide("title", { set: (t) => (documentTitle.value = t) });
</script>
