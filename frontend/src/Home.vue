<script setup>
import { ref, computed, onMounted } from 'vue'
import { api, LEVELS, MODES, fmtDate } from './api'
defineEmits(['enter', 'logout'])
const tab = ref('games'), list = ref([]), master = ref([]), show = ref(false), msg = ref('')
const f = ref({ name: '', play_date: new Date().toLocaleDateString('en-CA'), mode: 'regular' })
const np = ref({ id: '', nama: '', initial_level: '' }), psearch = ref('')
const shown = computed(() => master.value.filter(p => (p.nama + ' ' + p.id).toLowerCase().includes(psearch.value.trim().toLowerCase())))
async function load() {
  list.value = await api('GET', '/sessions'); master.value = await api('GET', '/players')
  np.value.id = String(Math.max(0, ...master.value.map(p => parseInt(p.id) || 0)) + 1).padStart(4, '0')
}
onMounted(() => load().catch(e => (msg.value = e.message)))
async function create() { await api('POST', '/sessions', f.value); show.value = false; f.value.name = ''; await load() }
async function del(g) { if (confirm(`Delete "${g.name}" and all its matches?`)) { await api('DELETE', '/sessions/' + g.id); await load() } }
async function addPlayer() { await api('POST', '/players', np.value); np.value.nama = ''; np.value.initial_level = ''; await load() }
const saveMaster = p => api('PUT', `/players/${p.id}`, { id: p.id, nama: p.nama, initial_level: p.initial_level || '' })
async function delMaster(p) { if (confirm(`Delete ${p.nama}?`)) { await api('DELETE', `/players/${p.id}`); await load() } }
const devices = ref([])
const loadDevices = async () => { try { devices.value = await api('GET', '/devices') } catch (e) { msg.value = e.message } }
const dev = ua => {
  const os = /iPhone|iPad/.test(ua) ? 'iPhone/iPad' : /Android/.test(ua) ? 'Android' : /Mac OS/.test(ua) ? 'Mac' : /Windows/.test(ua) ? 'Windows' : 'Unknown device'
  const br = /Edg\//.test(ua) ? 'Edge' : /CriOS|Chrome/.test(ua) ? 'Chrome' : /Safari/.test(ua) ? 'Safari' : /Firefox/.test(ua) ? 'Firefox' : 'browser'
  return `${os} · ${br}`
}
const ago = iso => { const m = Math.max(0, Math.round((Date.now() - new Date(iso)) / 60000)); return m < 1 ? 'just now' : m < 60 ? m + ' min ago' : m < 1440 ? Math.round(m / 60) + ' h ago' : Math.round(m / 1440) + ' d ago' }
async function kick(d) { await api('DELETE', '/devices/' + d.id); await loadDevices() }
async function kickOthers() { if (confirm('Log out every other device?')) { await api('POST', '/devices/logout-others'); await loadDevices() } }
</script>
<template>
  <div class="op">
    <nav>
      <button :class="{ on: tab === 'games' }" @click="tab = 'games'">Games</button>
      <button :class="{ on: tab === 'players' }" @click="tab = 'players'">Players</button>
      <button :class="{ on: tab === 'account' }" @click="tab = 'account'; loadDevices()">Account</button>
    </nav>
    <p v-if="msg" class="msg">{{ msg }}</p>
    <template v-if="tab === 'games'">
      <header class="hero">
        <div><h1>Banting Raket</h1><p class="tag">Play together. Get better.</p></div>
        <div class="acts"><button class="btn sm" @click="$emit('logout')">Log out</button><button class="btn primary" @click="show = true">+ New game</button></div>
      </header>
      <p v-if="!list.length" class="empty">No games yet. Create one to start.</p>
      <section v-for="g in list" :key="g.id" class="card gcard">
        <div>
          <h3>{{ g.name }}</h3>
          <p><span class="mchip">{{ MODES[g.mode] }}</span>{{ fmtDate(g.play_date) }} · {{ g.players }} players · {{ g.matches }} matches</p>
        </div>
        <div class="acts"><button class="btn ghost sm" @click="del(g)">Delete</button><button class="btn primary" @click="$emit('enter', g.id)">Enter →</button></div>
      </section>
    </template>
    <section v-else-if="tab === 'players'" class="card">
      <div class="bar"><h2>Master players ({{ master.length }})</h2></div>
      <form class="add" @submit.prevent="addPlayer">
        <input v-model="np.id" placeholder="ID" required /><input v-model="np.nama" placeholder="Name" required />
        <select v-model="np.initial_level"><option value="">No level</option><option v-for="l in LEVELS" :key="l">{{ l }}</option></select>
        <button class="btn primary">Add player</button>
      </form>
      <input v-model="psearch" class="search" type="search" placeholder="Search master players…" />
      <table class="tbl">
        <thead><tr><th>ID</th><th>Nama</th><th>Level</th><th></th></tr></thead>
        <tbody><tr v-for="p in shown" :key="p.id">
          <td data-l="ID">{{ p.id }}</td>
          <td data-l="Nama"><input v-model="p.nama" @change="saveMaster(p)" /></td>
          <td data-l="Level"><select v-model="p.initial_level" @change="saveMaster(p)"><option :value="null">-</option><option v-for="l in LEVELS" :key="l">{{ l }}</option></select></td>
          <td><button class="btn ghost sm" @click="delMaster(p)">Delete</button></td>
        </tr></tbody>
      </table>
    </section>
    <section v-else class="card">
      <div class="bar"><h2>Logged-in devices</h2></div>
      <p class="empty" style="margin-top:0">Every phone or computer that is logged in is listed here. A login lasts 7 days. The view-only share link never logs anyone in. If you see a device you don't know, log it out and change the password.</p>
      <ul class="devs">
        <li v-for="d in devices" :key="d.id">
          <div><b>{{ dev(d.ua) }}</b> <span v-if="d.current" class="mchip">This device</span><br /><small>Last active {{ ago(d.last_seen) }} · logged in {{ ago(d.created) }}</small></div>
          <button v-if="!d.current" class="btn ghost sm" @click="kick(d)">Log out</button>
        </li>
      </ul>
      <div class="acts"><button class="btn" :disabled="devices.length < 2" @click="kickOthers">Log out all other devices</button></div>
    </section>
    <div v-if="show" class="modal" @click.self="show = false">
      <form @submit.prevent="create">
        <h2>New game</h2>
        <label>Name<input v-model="f.name" placeholder="e.g. Open Play Sabtu" required /></label>
        <label>Date<input type="date" v-model="f.play_date" required /></label>
        <label>System
          <select v-model="f.mode"><option v-for="(l, k) in MODES" :key="k" :value="k">{{ l }}</option></select>
        </label>
        <p class="empty" style="margin:0">
          {{ f.mode === 'regular' ? 'Matches by level and fewest games. Ranked by wins.' : f.mode === 'team' ? 'Two groups of doubles teams (mixed, women, men). Group vs group, ranked by team and by group.' : 'Partners rotate every round. Ranked by total points scored.' }}
        </p>
        <div class="acts"><button type="button" class="btn" @click="show = false">Cancel</button><button class="btn primary">Create</button></div>
      </form>
    </div>
  </div>
</template>
