<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <svg
    v-if="name === graphTypes.sankey"
    v-bind="$attrs"
    preserveAspectRatio="xMidYMid meet"
    viewBox="0 0 24 24"
    style="vertical-align: -0.125em"
  >
    <path
      fill="currentColor"
      d="M20 4v2H4V4H2v8h2v-2c4.16 0 5.92 2.11 7.77 4.34S15.65 19 20 19v2h2v-6h-2v2c-3.41 0-4.93-1.83-6.69-3.94C11.34 10.69 9.1 8 4 8h16v2h2V4Z"
    />
  </svg>
  <svg
    v-if="name === graphTypes.stacked"
    v-bind="$attrs"
    preserveAspectRatio="xMidYMid meet"
    viewBox="0 0 24 24"
    style="vertical-align: -0.125em"
  >
    <path
      fill="currentColor"
      d="m8 17l3.39-4.66c.33-.46.98-.55 1.42-.2L21 18.5v.5c0 .55-.45 1-1 1H4c-.55 0-1-.45-1-1v-5.72c.22 0 .44.07.62.22L8 17zm-5-6c.44 0 .88.15 1.25.44l3.37 2.69l2.77-3.81c.66-.91 1.95-1.1 2.85-.4L21 15.97V8c0-.55-.45-1-1-1h-3l-4.18-3.34a.998.998 0 0 0-1.44.2L7 10L3.6 7.45c-.18-.13-.39-.2-.6-.2V11z"
    />
  </svg>
  <svg
    v-if="name === graphTypes.stacked100"
    v-bind="$attrs"
    preserveAspectRatio="xMidYMid meet"
    viewBox="0 0 24 24"
    style="vertical-align: -0.125em"
  >
    <path
      fill="currentColor"
      d="M 3.6757812 3.5546875 C 3.6757812 3.5546875 2.9810386 3.5136625 3 4.2148438 L 3 5.8183594 L 7.3339844 8.6367188 L 17.410156 5.7304688 L 21 9.6113281 L 21 4.2148438 C 21.00758 3.6537745 20.328125 3.5546875 20.328125 3.5546875 L 3.6757812 3.5546875 z M 16.779297 7.9941406 L 7.0117188 10.810547 L 3 8.203125 L 3 15.542969 L 10.482422 11.994141 L 17.167969 16.132812 L 21 14.996094 L 21 12.498047 L 20.970703 12.525391 L 16.779297 7.9941406 z M 10.355469 14.267578 L 3 17.755859 L 3 19 C 3 19.549998 3.450001 20 4 20 L 20 20 C 20.549998 20 21 19.57673 21 19 L 21 17.083984 L 16.878906 18.306641 L 10.355469 14.267578 z"
    />
  </svg>
  <svg
    v-if="name === graphTypes.lines"
    v-bind="$attrs"
    preserveAspectRatio="xMidYMid meet"
    viewBox="0 0 32 32"
    style="vertical-align: -0.125em"
  >
    <path
      fill="currentColor"
      d="m27.219 5.375l-3.5 4.375l-3.282-1.656l-.78-.375l-.5.718l-3.407 5.126l-3.156-2.376L12 10.75l-.594.438l-3.625 2.718l-3.531-.875l-.5 1.938l4 1l.469.125l.375-.282L12 13.25l3.406 2.563l.844.624l.594-.875l3.5-5.25l3.218 1.594l.72.344l4.5-5.625zm-7.157 12.938l-.843 1.062l-3.407 4.25l-3.218-2.438L12 20.75l-.594.438l-3.5 2.625l-3.468-1.72l-.875 1.813l4 2l.53.25l.5-.343L12 23.25l3.406 2.563l.781.562l.594-.75l3.125-3.906l3.25 4.843l.782 1.125l.843-1.062l4-5l-1.562-1.25l-3.125 3.906l-3.25-4.843z"
    />
  </svg>
  <svg
    v-if="name === graphTypes.grid"
    v-bind="$attrs"
    preserveAspectRatio="xMidYMid meet"
    viewBox="0 0 24 24"
    style="vertical-align: -0.125em"
  >
    <path
      fill="currentColor"
      d="M10 3H4a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V4a1 1 0 0 0-1-1zM9 9H5V5h4v4zm5 2h6a1 1 0 0 0 1-1V4a1 1 0 0 0-1-1h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1zm1-6h4v4h-4V5zM3 20a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1v-6a1 1 0 0 0-1-1H4a1 1 0 0 0-1 1v6zm2-5h4v4H5v-4zm8 5a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1v-6a1 1 0 0 0-1-1h-6a1 1 0 0 0-1 1v6zm2-5h4v4h-4v-4z"
    />
  </svg>
</template>

<script lang="ts">
export default {
  inheritAttrs: false,
};
</script>

<script lang="ts" setup>
import { graphTypes } from "./graphtypes";

defineProps<{ name: string }>();
</script>
