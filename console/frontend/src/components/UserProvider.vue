<template>
  <slot></slot>
</template>

<script setup>
import { provide, computed, readonly } from "vue";
import { useFetch } from "@vueuse/core";

const { data } = useFetch("/api/v0/console/user/info").get().json();

provide("user", {
  isAuthenticated: computed(() => data.value !== null),
  user: readonly(data),
});
</script>
