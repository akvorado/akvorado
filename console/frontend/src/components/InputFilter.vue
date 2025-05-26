<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <InputBase v-slot="{ childClass }" :error="error" v-bind="$attrs">
    <div ref="elEditor" :class="childClass"></div>
  </InputBase>
  <InputListBox
    v-model="selectedSavedFilter"
    v-bind="$attrs"
    :items="savedFilters"
    filter="description"
    label="Saved filters"
  >
    <template #item="{ description, shared, user, id }">
      <div class="flex w-full items-center justify-between">
        <div class="grow truncate">
          {{ description }}
          <span
            v-if="shared && user != currentUser?.login"
            class="ml-0 block text-xs italic text-gray-500 dark:text-gray-400 sm:max-lg:ml-1 sm:max-lg:inline"
          >
            <span v-if="user == '__system'">Shared by {{ user }}</span>
            <span v-else>System filter</span>
          </span>
        </div>
        <TrashIcon
          v-if="user == currentUser?.login"
          class="inline h-4 w-4 shrink cursor-pointer hover:text-blue-700 dark:hover:text-white"
          @click.stop.prevent="deleteFilter(id)"
        />
      </div>
    </template>
    <template #nomatch="{ query }">
      <div class="flex items-center justify-between gap-2">
        <span class="grow truncate">
          Save as “<span class="truncate">{{ query }}</span
          >”...
        </span>
        <div class="flex shrink items-center gap-1">
          <InputButton
            type="alternative"
            size="small"
            title="Share with others"
            @click.stop.prevent="
              addFilter({ description: query, shared: true })
            "
          >
            <EyeIcon class="h-3 w-3" />
          </InputButton>
          <InputButton
            type="primary"
            size="small"
            title="Keep private"
            @click.stop.prevent="
              addFilter({ description: query, shared: false })
            "
          >
            <EyeOffIcon class="h-3 w-3" />
          </InputButton>
        </div>
      </div>
    </template>
  </InputListBox>
</template>

<script lang="ts" setup>
import { ref, inject, watch, computed, onMounted, onBeforeUnmount } from "vue";
import { useFetch } from "@vueuse/core";
import { TrashIcon, EyeIcon, EyeOffIcon } from "@heroicons/vue/solid";
import InputBase from "@/components/InputBase.vue";
import InputListBox from "@/components/InputListBox.vue";
import InputButton from "@/components/InputButton.vue";
import { ThemeKey } from "@/components/ThemeProvider.vue";
import { UserKey } from "@/components/UserProvider.vue";

import {
  EditorState,
  StateEffect,
  Compartment,
  type Extension,
} from "@codemirror/state";
import { EditorView, keymap, placeholder } from "@codemirror/view";
import { syntaxHighlighting, HighlightStyle } from "@codemirror/language";
import { standardKeymap, history } from "@codemirror/commands";
import { linter } from "@codemirror/lint";
import { autocompletion, acceptCompletion } from "@codemirror/autocomplete";
import { tags as t } from "@lezer/highlight";
import {
  filterLanguage,
  filterCompletion,
  filterLinterSource,
} from "@/codemirror/lang-filter";
import { isEqual } from "lodash-es";

const props = defineProps<{
  modelValue: ModelType;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: typeof props.modelValue];
  submit: [];
}>();

const { isDark } = inject(ThemeKey)!;
const { user: currentUser } = inject(UserKey)!;

// # Saved filters
type SavedFilter = {
  id: number;
  user: string;
  shared: boolean;
  description: string;
  content: string;
};

const selectedSavedFilter = ref<SavedFilter | null>(null);
const { data: rawSavedFilters, execute: refreshSavedFilters } = useFetch(
  `/api/v0/console/filter/saved`,
).json<{
  filters: Array<SavedFilter>;
}>();
const savedFilters = computed(() => rawSavedFilters.value?.filters ?? []);
watch(selectedSavedFilter, (filter) => {
  if (!filter?.content) return;
  expression.value = filter.content;
  selectedSavedFilter.value = null;
});

const deleteFilter = async (id: SavedFilter["id"]) => {
  try {
    await fetch(`/api/v0/console/filter/saved/${id}`, { method: "DELETE" });
  } finally {
    refreshSavedFilters();
  }
};
const addFilter = async ({
  description,
  shared,
}: Pick<SavedFilter, "description" | "shared">) => {
  try {
    await fetch(`/api/v0/console/filter/saved`, {
      method: "POST",
      body: JSON.stringify({ description, shared, content: expression.value }),
    });
  } finally {
    refreshSavedFilters();
  }
};

// # Editor
const elEditor = ref<HTMLDivElement | null>(null);
const expression = ref(""); // Keep in sync with modelValue.expression
const error = ref(""); // Keep in sync with modelValue.errors
let component:
  | { view: EditorView; state: EditorState }
  | { view: null; state: null } = {
  view: null,
  state: null,
};
watch(
  () => props.modelValue,
  (model) => {
    if (model) expression.value = model.expression;
  },
  { immediate: true },
);
watch(
  () => ({ expression: expression.value, errors: !!error.value }),
  (value) => {
    if (!isEqual(props.modelValue, value)) {
      emit("update:modelValue", value);
    }
  },
  { immediate: true },
);

// https://github.com/surmon-china/vue-codemirror/blob/59598ff72327ab6c5ee70a640edc9e2eb2518775/src/codemirror.ts#L52
const rerunExtension = () => {
  const compartment = new Compartment();
  const run = (view: EditorView, extension: Extension) => {
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
    ]),
  ),
  /* Theme is in tailwind.css */
  EditorView.theme({}, { dark: isDark.value }),
]);

const submitFilter = (_: EditorView): boolean => {
  emit("submit");
  return true;
};

onMounted(() => {
  // Create Code mirror instance
  const state = EditorState.create({
    doc: props.modelValue?.expression ?? "",
    extensions: [
      filterLanguage(),
      filterCompletion(),
      autocompletion({ icons: false }),
      linter(async (v) => {
        const diags = await filterLinterSource(v);
        error.value = diags.length > 0 ? "Invalid filter expression" : "";
        return diags;
      }),
      keymap.of([
        ...standardKeymap.filter((b) => b.key !== "Mod-a"),
        { key: "Tab", run: acceptCompletion },
        { key: "Ctrl-Enter", run: submitFilter },
        { key: "Cmd-Enter", run: submitFilter },
      ]),
      history(),
      placeholder("Filter expression"),
      EditorView.lineWrapping,
      EditorView.updateListener.of((viewUpdate) => {
        if (viewUpdate.docChanged) {
          expression.value = viewUpdate.state.doc.toString();
        }
        if (viewUpdate.focusChanged) {
          if (!viewUpdate.view.hasFocus) {
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
  const view = new EditorView({
    state: state,
    parent: elEditor.value!,
  });
  component = {
    state,
    view,
  };

  watch(
    expression,
    (expression) => {
      if (expression !== component.view?.state.doc.toString()) {
        component.view?.dispatch({
          changes: {
            from: 0,
            to: component.view.state.doc.length,
            insert: expression,
          },
        });
      }
    },
    { immediate: true },
  );

  // Dynamic extensions
  const dynamicExtensions = rerunExtension();
  const extensions = computed(() => [...filterTheme.value]);
  watch(
    extensions,
    (extensions) => {
      const exts = extensions.filter((e) => !!e);
      if (component.view !== null) dynamicExtensions(component.view, exts);
    },
    { immediate: true },
  );
});
onBeforeUnmount(() => component.view?.destroy());
</script>

<script lang="ts">
export default {
  inheritAttrs: false,
};
export type ModelType = {
  expression: string;
  errors?: boolean;
} | null;
</script>
