<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, shallowRef, nextTick } from 'vue'
import { EditorState } from '@codemirror/state'
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, drawSelection } from '@codemirror/view'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { syntaxHighlighting, defaultHighlightStyle, bracketMatching, foldGutter, indentOnInput } from '@codemirror/language'
import { yaml } from '@codemirror/lang-yaml'
import { oneDark } from '@codemirror/theme-one-dark'
import { useAppStore } from '@/stores/app'

const props = withDefaults(defineProps<{
  modelValue?: string
  readonly?: boolean
  placeholder?: string
}>(), {
  modelValue: '',
  readonly: false,
  placeholder: '',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const app = useAppStore()
const editorRef = ref<HTMLDivElement>()
const view = shallowRef<EditorView>()
const skipEmit = ref(false)
const ready = ref(false)

function buildExtensions(): any[] {
  const exts: any[] = [
    lineNumbers(),
    highlightActiveLineGutter(),
    highlightActiveLine(),
    drawSelection(),
    indentOnInput(),
    bracketMatching(),
    foldGutter(),
    history(),
    keymap.of([...defaultKeymap, ...historyKeymap]),
    yaml(),
    syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
    EditorView.lineWrapping,
    EditorState.tabSize.of(2),
  ]

  if (app.theme === 'dark') exts.push(oneDark)

  if (props.readonly) {
    exts.push(EditorState.readOnly.of(true))
    exts.push(EditorView.editable.of(false))
  } else {
    exts.push(EditorView.updateListener.of((u) => {
      if (u.docChanged && !skipEmit.value) {
        emit('update:modelValue', u.state.doc.toString())
      }
    }))
  }

  if (props.placeholder) {
    exts.push(EditorView.theme({
      '.cm-content:empty::before': {
        content: `"${props.placeholder}"`,
        color: '#86868b',
        fontStyle: 'italic',
      },
    }))
  }

  return exts
}

function mountEditor(doc: string) {
  if (!editorRef.value) return
  view.value?.destroy()
  view.value = new EditorView({
    state: EditorState.create({ doc, extensions: buildExtensions() }),
    parent: editorRef.value,
  })
  ready.value = true
}

function setDoc(val: string) {
  if (!view.value) return
  const cur = view.value.state.doc.toString()
  if (val === cur) return
  skipEmit.value = true
  view.value.dispatch({
    changes: { from: 0, to: view.value.state.doc.length, insert: val ?? '' },
  })
  skipEmit.value = false
}

onMounted(() => {
  mountEditor(props.modelValue ?? '')
})

watch(() => props.modelValue, (val) => {
  nextTick(() => setDoc(val ?? ''))
})

watch(() => app.theme, () => {
  const doc = view.value?.state.doc.toString() ?? ''
  mountEditor(doc)
})

onBeforeUnmount(() => {
  view.value?.destroy()
})

defineExpose({
  getValue: () => view.value?.state.doc.toString() ?? '',
  setValue: setDoc,
})
</script>

<template>
  <div ref="editorRef" class="yaml-editor" />
</template>

<style scoped>
.yaml-editor {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.yaml-editor :deep(.cm-editor) {
  height: 100%;
  font-family: var(--font-mono);
  font-size: 0.85rem;
}

.yaml-editor :deep(.cm-editor.cm-focused) {
  outline: none;
}

.yaml-editor :deep(.cm-gutters) {
  background: var(--bg-input);
  border-right: 0.5px solid var(--border);
  color: var(--text-muted);
}

.yaml-editor :deep(.cm-content) {
  padding: 8px 0;
}

.yaml-editor :deep(.cm-activeLineGutter) {
  background: var(--bg-hover);
}

.yaml-editor :deep(.cm-activeLine) {
  background: rgba(128,128,128,0.06);
}

.yaml-editor :deep(.cm-selectionBackground) {
  background: rgba(10,132,255,0.2) !important;
}

.yaml-editor :deep(.cm-cursor) {
  border-left-color: var(--accent);
}
</style>
