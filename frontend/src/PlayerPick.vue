<script setup>
import { ref, computed, watch } from 'vue'
const props = defineProps({ modelValue: String, options: { type: Array, default: () => [] }, placeholder: String })
const emit = defineEmits(['update:modelValue'])
const q = ref(''), open = ref(false), act = ref(0)
const selected = computed(() => props.options.find(o => o.id === props.modelValue))
const shown = computed(() => props.options.filter(o => o.label.toLowerCase().includes(q.value.trim().toLowerCase())))
watch(q, () => (act.value = 0))
const text = computed(() => (open.value ? q.value : selected.value?.label || ''))
function pick(o) { emit('update:modelValue', o.id); q.value = ''; open.value = false }
function clear() { emit('update:modelValue', ''); q.value = '' }
function onKey(e) {
  const n = shown.value.length
  if (e.key === 'ArrowDown' && n) { e.preventDefault(); open.value = true; act.value = (act.value + 1) % n }
  else if (e.key === 'ArrowUp' && n) { e.preventDefault(); act.value = (act.value - 1 + n) % n }
  else if (e.key === 'Enter') { e.preventDefault(); if (shown.value[act.value]) pick(shown.value[act.value]) }
  else if (e.key === 'Escape') open.value = false
}
</script>
<template>
  <div class="pick-wrap">
    <input :value="text" :placeholder="selected?.label || placeholder" autocomplete="off"
      @input="q = $event.target.value; open = true" @focus="open = true; q = ''" @blur="open = false" @keydown="onKey" />
    <button v-if="modelValue" type="button" class="x" aria-label="Clear" @mousedown.prevent="clear">×</button>
    <ul v-if="open" class="list">
      <li v-for="(o, i) in shown" :key="o.id">
        <button type="button" :class="{ on: i === act }" @mouseenter="act = i" @mousedown.prevent="pick(o)">{{ o.label }}<small>{{ o.sub }}</small></button>
      </li>
      <li v-if="!shown.length" class="none">No match</li>
    </ul>
  </div>
</template>
