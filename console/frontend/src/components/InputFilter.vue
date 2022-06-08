<template>
  <InputBase v-slot="{ childClass }" :error="error">
    <div ref="el" :class="childClass"></div>
  </InputBase>
</template>

<script setup>
const props = defineProps({
  modelValue: {
    // expression: filter expression
    // errors: boolean if there are errors
    type: Object,
    required: true,
  },
});
const emit = defineEmits(["update:modelValue"]);

import { ref, inject, watch, computed, onMounted, onBeforeUnmount } from "vue";
import InputBase from "@/components/InputBase.vue";
const { isDark } = inject("theme");

import { EditorState, StateEffect, Compartment } from "@codemirror/state";
import { EditorView, keymap, placeholder } from "@codemirror/view";
import { syntaxHighlighting, HighlightStyle } from "@codemirror/language";
import { standardKeymap } from "@codemirror/commands";
import { linter } from "@codemirror/lint";
import {
  autocompletion,
  acceptCompletion,
  startCompletion,
} from "@codemirror/autocomplete";
import { tags as t } from "@lezer/highlight";
import {
  filterLanguage,
  filterCompletion,
  filterLinterSource,
} from "@/codemirror/lang-filter";
import { isEqual } from "lodash-es";

const el = ref(null);
const expression = ref(""); // Keep in sync with modelValue.expression
const error = ref(""); // Keep in sync with modelValue.errors
const component = {
  view: null,
  state: null,
};
watch(
  () => props.modelValue,
  (model) => (expression.value = model.expression),
  { immediate: true }
);
watch(
  () => ({ expression: expression.value, errors: !!error.value }),
  (value) => {
    if (!isEqual(props.modelValue, value)) {
      emit("update:modelValue", value);
    }
  },
  { immediate: true }
);

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

// Theme for the filtering language
const filterTheme = computed(() => [
  syntaxHighlighting(
    HighlightStyle.define([
      { tag: t.propertyName, color: isDark.value ? "#fb660a" : "#008800" },
      { tag: t.string, color: isDark.value ? "#ff0086" : "#880000" },
      { tag: t.comment, color: isDark.value ? "#7d8799" : "#4f4f4f" },
      { tag: t.operator, color: isDark.value ? "#00a3ff" : "#333399" },
    ])
  ),
  /* Theme is in tailwind.css */
  EditorView.theme({}, { dark: isDark.value }),
]);

onMounted(() => {
  // Create Code mirror instance
  component.state = EditorState.create({
    doc: props.modelValue.expression,
    extensions: [
      filterLanguage(),
      filterCompletion(),
      autocompletion({ icons: false }),
      linter(async (v) => {
        const diags = await filterLinterSource(v);
        error.value = diags.length > 0 ? "Invalid filter expression" : "";
        return diags;
      }),
      keymap.of([...standardKeymap, { key: "Tab", run: acceptCompletion }]),
      placeholder("Filter expression"),
      EditorView.lineWrapping,
      EditorView.updateListener.of((viewUpdate) => {
        if (viewUpdate.docChanged) {
          expression.value = viewUpdate.state.doc.toString();
        }
        if (viewUpdate.focusChanged) {
          if (viewUpdate.view.hasFocus) startCompletion(viewUpdate.view);
          else {
            // Trim spaces
            const index = viewUpdate.state.doc.toString().search(/\s+$/);
            if (index !== -1) {
              viewUpdate.view.dispatch({
                changes: {
                  from: index,
                  to: viewUpdate.state.doc.length,
                },
              });
            }
          }
        }
      }),
    ],
  });
  component.view = new EditorView({
    state: component.state,
    parent: el.value,
  });

  watch(
    expression,
    (expression) => {
      if (expression !== component.view.state.doc.toString()) {
        component.view.dispatch({
          changes: {
            from: 0,
            to: component.view.state.doc.length,
            insert: expression,
          },
        });
      }
    },
    { immediate: true }
  );

  // Dynamic extensions
  const dynamicExtensions = rerunExtension();
  const extensions = computed(() => [...filterTheme.value]);
  watch(
    extensions,
    (extensions) => {
      const exts = extensions.filter((e) => !!e);
      dynamicExtensions(component.view, exts);
    },
    { immediate: true }
  );
});
onBeforeUnmount(() => component.view?.destroy());
</script>
