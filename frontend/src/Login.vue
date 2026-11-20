<script setup>
import { ref } from 'vue'
const emit = defineEmits(['done'])
const u = ref(''), p = ref(''), err = ref(''), busy = ref(false), show = ref(false)
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
      <div class="pw">
        <input v-model="p" :type="show ? 'text' : 'password'" placeholder="Password" autocomplete="current-password" required />
        <button type="button" :aria-label="show ? 'Hide password' : 'Show password'" @click="show = !show">
          <svg v-if="!show" viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" /><circle cx="12" cy="12" r="3" /></svg>
          <svg v-else viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" /><line x1="1" y1="1" x2="23" y2="23" /></svg>
        </button>
      </div>
      <button class="btn primary" :disabled="busy">{{ busy ? 'Signing in…' : 'Log in' }}</button>
    </form>
  </div>
</template>
