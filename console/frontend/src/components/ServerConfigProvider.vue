<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <slot></slot>
</template>

<script lang="ts" setup>
import { provide, readonly } from "vue";
import { useFetch } from "@vueuse/core";
import type { graphTypes } from "../views/VisualizePage/graphtypes";

const { data } = useFetch("/api/v0/console/configuration")
  .get()
  .json<ServerConfig>();

provide(ServerConfigKey, readonly(data));
</script>

<script lang="ts">
import type { InjectionKey, Ref } from "vue";

type ServerConfig = {
  version: string;
  defaultVisualizeOptions: {
    graphType: keyof typeof graphTypes;
    start: string;
    end: string;
    filter: string;
    dimensions: string[];
    limit: number;
  };
  dimensions: string[];
  dimensionsLimit: number;
  homepageTopWidgets: string[];
};

export const ServerConfigKey: InjectionKey<Readonly<Ref<ServerConfig>>> =
  Symbol();
</script>
