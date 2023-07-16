<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <slot></slot>
</template>

<script lang="ts" setup>
import { provide, readonly, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useFetch } from "@vueuse/core";

const { data, execute } = useFetch("/api/v0/console/user/info", {
  immediate: false,
  onFetchError(ctx) {
    if (ctx.response?.status === 401) {
      // TODO: avoid component flash.
      router.replace({ name: "401", query: { redirect: route.path } });
    }
    return ctx;
  },
})
  .get()
  .json<UserInfo>();

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
  { immediate: true },
);

provide(UserKey, {
  user: readonly(data),
});
</script>

<script lang="ts">
import type { InjectionKey, Ref } from "vue";

export type UserInfo = {
  login: string;
  name?: string;
  email?: string;
  "logout-url"?: string;
};
export const UserKey: InjectionKey<{
  user: Readonly<Ref<UserInfo>>;
}> = Symbol();
</script>
