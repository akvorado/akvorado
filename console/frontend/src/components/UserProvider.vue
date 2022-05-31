<template>
  <slot></slot>
</template>

<script setup>
import { provide, readonly, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useFetch } from "@vueuse/core";

const { data, execute } = useFetch("/api/v0/console/user/info", {
  immediate: false,
  onFetchError(ctx) {
    if (ctx.response.status === 401) {
      // TODO: avoid component flash.
      router.replace({ name: "401", query: { redirect: route.path } });
    }
    return ctx;
  },
})
  .get()
  .json();

// Handle verification on route change.
const route = useRoute();
const router = useRouter();
watch(
  route,
  (to) => {
    if (!to.meta.notAuthenticated) {
      execute(false);
    }
  },
  { immediate: true }
);

provide("user", {
  user: readonly(data),
});
</script>
