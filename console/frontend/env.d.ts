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
