<template>
  <InputComponent v-slot="{ childClass }">
    <div ref="el" :class="childClass"></div>
  </InputComponent>
</template>

<script setup>
const props = defineProps({
  modelValue: {
    type: String,
    required: true,
  },
});
const emit = defineEmits(["update:modelValue"]);

import { ref, inject, watch, computed, onMounted, onBeforeUnmount } from "vue";
import InputComponent from "@/components/InputComponent.vue";
const { isDark } = inject("theme");

import { EditorState, StateEffect, Compartment } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { syntaxHighlighting, HighlightStyle } from "@codemirror/language";
import { defaultKeymap } from "@codemirror/commands";
import { tags as t } from "@lezer/highlight";
import { filter } from "@/codemirror/lang-filter";

const el = ref(null);
const component = {
  view: null,
  state: null,
};

// https://github.com/surmon-china/vue-codemirror/blob/59598ff72327ab6c5ee70a640edc9e2eb2518775/src/codemirror.ts#L52
const rerunExtension = () => {
  const compartment = new Compartment();
  const run = (view, extension) => {
    if (compartment.get(view.state)) {
      // reconfigure
      view.dispatch({ effects: compartment.reconfigure(extension) });
    } else {
      // inject
      view.dispatch({
        effects: StateEffect.appendConfig.of(compartment.of(extension)),
      });
    }
  };
  return run;
};

onMounted(() => {
  // Create Code mirror instance
  component.state = EditorState.create({
    doc: props.modelValue,
    extensions: [
      keymap.of(defaultKeymap),
      filter(),
      EditorView.lineWrapping,
      EditorView.updateListener.of((viewUpdate) => {
        if (viewUpdate.docChanged) {
          const doc = viewUpdate.state.doc.toString();
          if (doc !== props.modelValue) {
            emit("update:modelValue", doc);
          }
        }
      }),
    ],
  });
  component.view = new EditorView({
    state: component.state,
    parent: el.value,
  });

  // Update it as the model changes
  watch(
    () => props.modelValue,
    (value) => {
      if (value !== component.view.state.doc.toString()) {
        component.view.dispatch({
          changes: {
            from: 0,
            to: component.view.state.doc.length,
            insert: value,
          },
        });
      }
    },
    { immediate: true }
  );

  // Dynamic extensions
  const dynamicExtensions = rerunExtension();
  const extensions = computed(() => [
    syntaxHighlighting(
      HighlightStyle.define([
        { tag: t.propertyName, color: isDark.value ? "#fb660a" : "#008800" },
        { tag: t.string, color: isDark.value ? "#ff0086" : "#880000" },
        { tag: t.comment, color: isDark.value ? "#7d8799" : "#4f4f4f" },
        { tag: t.operator, color: isDark.value ? "#00a3ff" : "#333399" },
      ])
    ),
    EditorView.theme(
      {
        "&.cm-editor.cm-focused": {
          outline: "none",
        },
      },
      { dark: isDark.value }
    ),
  ]);
  watch(
    extensions,
    (extensions) => {
      const exts = extensions.filter((e) => !!e);
      dynamicExtensions(component.view, exts);
    },
    { immediate: true }
  );
});
onBeforeUnmount(() => component.view.destroy());
</script>
