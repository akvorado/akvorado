// Allow use of aria-current on all components
declare module "@vue/runtime-core" {
  interface AllowedComponentProps {
    "aria-current"?:
      | Booleanish
      | "page"
      | "step"
      | "location"
      | "date"
      | "time";
  }
}

export {};
