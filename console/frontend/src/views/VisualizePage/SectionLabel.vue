<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <p
    class="my-2 flex w-full flex-row flex-nowrap items-center justify-between gap-2"
  >
    <span class="text-sm font-semibold text-gray-900 dark:text-gray-400">
      <slot></slot>
    </span>
    <span class="shrink truncate text-xs text-gray-700 dark:text-gray-500">
      <slot name="hint"></slot>
    </span>
  </p>
</template>
