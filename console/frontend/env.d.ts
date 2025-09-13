// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: 0BSD
/// <reference types="vite/client" />

// Missing types for vue-resizer
declare module "vue-resizer" {
  import type { DefineComponent } from "vue";
  declare const ResizeRow: DefineComponent<{
    sliderWidth?: number;
    height?: number;
    width?: number | "auto";
    sliderColor?: string;
    sliderBgColor?: string;
    sliderHoverColor?: string;
    sliderBgHoverColor?: string;
  }>;
}

// Missing ES module exports for sugar-date
declare module "sugar-date" {
  const Sugar: sugarjs.Sugar;
  export = Sugar;
}

// Export TooltipCallbackDataParams from echarts
declare module "echarts/types/src/component/tooltip/TooltipView.d.ts" {
  export { TooltipCallbackDataParams };
}
