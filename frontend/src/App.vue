<script setup>
import { ref, computed, onMounted } from 'vue'
import Home from './Home.vue'
import Session from './Session.vue'
import Login from './Login.vue'
import Public from './Public.vue'
const hash = ref(location.hash)
addEventListener('hashchange', () => (hash.value = location.hash))
const pub = computed(() => (hash.value.startsWith('#/v/') ? hash.value.slice(4) : null)) // view-only link
const sid = computed(() => parseInt(hash.value.slice(2)) || null)
const authed = ref(null) // null = checking
onMounted(async () => { if (!pub.value) authed.value = (await fetch('/api/me')).ok })
addEventListener('unauth', () => (authed.value = false))
async function logout() { await fetch('/api/logout', { method: 'POST' }); authed.value = false; location.hash = '' }
const go = id => (location.hash = id ? `#/${id}` : '')
</script>
<template>
  <Public v-if="pub" :token="pub" />
  <template v-else>
    <p v-if="authed === null" class="empty" style="padding:24px">Loading…</p>
    <Login v-else-if="!authed" @done="authed = true" />
    <Session v-else-if="sid" :key="sid" :sid="sid" @back="go(null)" />
    <Home v-else @enter="go" @logout="logout" />
  </template>
</template>
