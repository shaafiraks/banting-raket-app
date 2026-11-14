<script setup>
import { ref } from 'vue'
const emit = defineEmits(['done'])
const u = ref(''), p = ref(''), err = ref(''), busy = ref(false)
async function go() {
  busy.value = true; err.value = ''
  try {
    const r = await fetch('/api/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: u.value, password: p.value }) })
    if (!r.ok) throw new Error(r.status === 401 ? 'Wrong username or password.' : await r.text())
    emit('done')
  } catch (e) { err.value = e.message } finally { busy.value = false }
}
</script>
<template>
  <div class="login">
    <form @submit.prevent="go">
      <div><h1>Banting Raket</h1><p class="tag">Admin login</p></div>
      <p v-if="err" class="msg">{{ err }}</p>
      <input v-model="u" placeholder="Username" autocomplete="username" autocapitalize="none" required />
      <input v-model="p" type="password" placeholder="Password" autocomplete="current-password" required />
      <button class="btn primary" :disabled="busy">{{ busy ? 'Signing in…' : 'Log in' }}</button>
    </form>
  </div>
</template>
