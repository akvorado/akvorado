<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="container flex flex-col items-center justify-center">
    <img
      class="block hover:animate-none motion-safe:animate-bounce"
      src="@/assets/images/akvorado.svg"
    />
    <h1 class="text-5xl font-bold">{{ error }}</h1>
  </div>
</template>

<script lang="ts" setup>
defineProps<{
  error: string;
}>();
</script>
