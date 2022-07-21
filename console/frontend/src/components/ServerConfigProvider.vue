<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <slot></slot>
</template>

<script setup>
import { provide, readonly } from "vue";
import { useFetch } from "@vueuse/core";

// TODO: handle error
const { data } = useFetch("/api/v0/console/configuration").get().json();

provide("server-configuration", readonly(data));
</script>
