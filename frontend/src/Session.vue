<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { api, LEVELS, MODES, fmtDate } from './api'
import PlayerPick from './PlayerPick.vue'
const props = defineProps({ sid: Number }); defineEmits(['back'])
const S = `/sessions/${props.sid}`
const LV = { A: 5, 'B+': 4, B: 3, 'B-': 2, C: 1, N: 0 } // unknown level = 2
const tab = ref('play'), info = ref({ name: '', play_date: '', mode: 'regular' }), master = ref([]), rows = ref([]), matches = ref([]), done = ref([]), teams = ref([])
const courts = ref(3), msg = ref(''), q = ref(''), open = ref(false), act = ref(0), newLevel = ref(''), man = ref(['', '', '', ''])
const tp = ref({ p1: '', p2: '', grp: 1, category: 'Mixed' })
let uid = 0
const mode = computed(() => info.value.mode)

async function load() {
  master.value = await api('GET', '/players')
  const s = await api('GET', S); info.value = s; courts.value = s.courts
  const old = new Map(rows.value.map(r => [r.id, r])) // keep on-court state across reloads
  rows.value = s.players.map(p => old.get(p.id) || { ...p, last: 0, st: null })
  done.value = await api('GET', S + '/matches'); teams.value = await api('GET', S + '/teams')
}
onMounted(() => load().catch(e => (msg.value = e.message)))

const nm = id => rows.value.find(r => r.id === id)?.nama || id
const nextId = () => String(Math.max(0, ...master.value.map(p => parseInt(p.id) || 0)) + 1).padStart(4, '0')
const found = computed(() => master.value.filter(p => p.nama.toLowerCase().includes(q.value.trim().toLowerCase())).slice(0, 8).map(p => ({ ...p, in: rows.value.some(r => r.id === p.id) })))
const canNew = computed(() => q.value.trim() && !master.value.some(p => p.nama.toLowerCase() === q.value.trim().toLowerCase()))
const paid = computed(() => rows.value.filter(r => r.payment !== 'belum').length)
const freeRows = computed(() => rows.value.filter(r => r.st === null))
const playing = computed(() => matches.value.filter(m => m.st === 'play'))
const slots = computed(() => courts.value - playing.value.length)
const ordered = computed(() => [...matches.value].sort((a, b) => (a.st === 'play' ? 0 : 1) - (b.st === 'play' ? 0 : 1) || a.id - b.id))
const editable = m => m.st === 'queue' && mode.value !== 'team'
const swapOptions = computed(() => rows.value.filter(r => r.st !== 'play'))

watch(q, () => (act.value = 0))
function onKey(e) {
  const n = found.value.length
  if (e.key === 'ArrowDown' && n) { e.preventDefault(); act.value = (act.value + 1) % n }
  else if (e.key === 'ArrowUp' && n) { e.preventDefault(); act.value = (act.value - 1 + n) % n }
  else if (e.key === 'Enter') { const p = found.value[act.value]; if (p && !p.in) checkIn(p.id) }
}
async function checkIn(id) { await api('POST', S + '/players', { player_id: id }); q.value = ''; newLevel.value = ''; await load() }
// Regular game: a new name is also saved to the master list. Team/Americano: guest for this game only.
async function addNew() {
  const nama = q.value.trim()
  if (mode.value === 'regular') { const id = nextId(); await api('POST', '/players', { id, nama, initial_level: newLevel.value }); await checkIn(id) }
  else { await api('POST', S + '/guests', { nama, level: newLevel.value }); q.value = ''; newLevel.value = ''; await load() }
}
const setPay = r => api('PUT', `${S}/players/${r.id}`, { payment: r.payment })
const setLevel = r => api('PUT', `${S}/players/${r.id}`, { level: r.level })
async function removeRow(r) { await api('DELETE', `${S}/players/${r.id}`); rows.value = rows.value.filter(x => x !== r) }
async function rename() { const n = info.value.name.trim(); if (!n) return load(); await api('PUT', S, { name: n }) }
async function setCourts(n) { courts.value = Math.max(1, Math.min(12, n)); await api('PUT', S, { courts: courts.value }) }

// ---------- match generation ----------
const val = r => LV[r.level] ?? 2
const pk = (a, b) => [a.id, b.id].sort().join('|')
const partnered = computed(() => { const m = new Map(); for (const d of done.value) for (const t of [d.p.slice(0, 2), d.p.slice(2)]) { const k = t.slice().sort().join('|'); m.set(k, (m.get(k) || 0) + 1) } return m })
function mk(t1, t2, extra = {}) { const m = { id: ++uid, st: 'queue', t1, t2, s1: null, s2: null, ...extra };[...t1, ...t2].forEach(p => (p.st = 'queue')); matches.value.push(m); return m }
function build() {
  const am = mode.value === 'americano'
  const s = freeRows.value.slice().sort((a, b) => a.games - b.games || a.last - b.last || Math.random() - 0.5)
  if (s.length < 4) return null
  const a = s[0]; let rest = s.slice(1)
  if (!am) for (let g = 1; g <= 5; g++) { rest = s.slice(1).filter(r => Math.abs(val(r) - val(a)) <= g); if (rest.length >= 3) break }
  const [p, q2, r, t] = [a, ...rest.slice(0, 3)].sort((x, y) => val(y) - val(x))
  const sum = x => x.reduce((n, y) => n + val(y), 0)
  const cost = o => (am ? 0 : Math.abs(sum(o[0]) - sum(o[1]))) + 3 * o.reduce((n, x) => n + (partnered.value.get(pk(...x)) || 0), 0)
  const best = [[[p, t], [q2, r]], [[p, q2], [r, t]], [[p, r], [q2, t]]].sort((x, y) => cost(x) - cost(y))[0]
  return mk(best[0], best[1])
}
// team group: group A team vs group B team; fewest games, no rematch, same category preferred
const has = (x, t) => x.includes(t.p1) && x.includes(t.p2)
const tgames = t => done.value.filter(d => has(d.p.slice(0, 2), t) || has(d.p.slice(2), t)).length
const met = (a, b) => done.value.some(d => { const x = d.p.slice(0, 2), y = d.p.slice(2); return (has(x, a) && has(y, b)) || (has(x, b) && has(y, a)) })
function buildTeam() {
  const busy = new Set(matches.value.flatMap(m => [m.a?.id, m.b?.id]))
  const free = teams.value.filter(t => !busy.has(t.id)); let best = null, bc = 1e9
  for (const a of free.filter(t => t.grp === 1)) for (const b of free.filter(t => t.grp === 2)) {
    const c = tgames(a) + tgames(b) + (met(a, b) ? 5 : 0) + (a.category !== b.category ? 3 : 0) + Math.random() * 0.5
    if (c < bc) { bc = c; best = [a, b] }
  }
  if (!best) return null
  const R = id => rows.value.find(r => r.id === id)
  return mk([R(best[0].p1), R(best[0].p2)], [R(best[1].p1), R(best[1].p2)], { a: best[0], b: best[1] })
}
const gen = () => (mode.value === 'team' ? buildTeam() : build())
const noneMsg = () => (mode.value === 'team' ? 'Need a free team in each group.' : 'Need at least 4 free players.')
function generate() { msg.value = ''; if (!gen()) msg.value = noneMsg() }
function generateRound() { msg.value = ''; let n = 0; while (gen()) n++; if (!n) msg.value = noneMsg() }
function start(m) { if (slots.value <= 0) return; m.st = 'play';[...m.t1, ...m.t2].forEach(p => (p.st = 'play')) }
function release(m) { matches.value = matches.value.filter(x => x !== m);[...m.t1, ...m.t2].forEach(p => (p.st = null)) }
// change one player of a generated match (swaps with the other match if that player is already in one)
function swap(m, side, i, id) {
  const old = m[side][i], nu = rows.value.find(r => r.id === id)
  if (!nu || nu === old) return
  const other = matches.value.find(x => x !== m && x.st === 'queue' && [...x.t1, ...x.t2].some(p => p.id === id))
  if (other) { const k = ['t1', 't2'].find(k => other[k].some(p => p.id === id)); other[k][other[k].findIndex(p => p.id === id)] = old } else old.st = null
  m[side][i] = nu; nu.st = 'queue'
}
const manOptions = i => rows.value.filter(r => r.st !== 'play' && !man.value.some((v, j) => j !== i && v === r.id))
function manual(playNow) {
  const [a, b, c, d] = man.value.map(id => rows.value.find(r => r.id === id))
  if (![a, b, c, d].every(Boolean)) { msg.value = 'Pick 4 players first.'; return }
  const ids = [a, b, c, d].map(x => x.id)
  matches.value.filter(m => m.st === 'queue' && [...m.t1, ...m.t2].some(p => ids.includes(p.id))).forEach(release)
  const m = mk([a, b], [c, d]); man.value = ['', '', '', '']; msg.value = ''
  if (playNow) start(m)
}
const valid = m => Number.isFinite(m.s1) && Number.isFinite(m.s2) && (mode.value === 'americano' || (m.s1 !== m.s2 && Math.max(m.s1, m.s2) >= 30))
async function finish(m) {
  const all = [...m.t1, ...m.t2], ids = all.map(p => p.id)
  await api('POST', S + '/matches', { players: ids, score1: m.s1, score2: m.s2 })
  done.value.push({ p: ids, s1: m.s1, s2: m.s2 }); all.forEach(p => { p.games++; p.last = Date.now() }); release(m)
}
const copy = m => navigator.clipboard.writeText(`${m.t1.map(p => p.nama).join(' & ')}\nvs\n${m.t2.map(p => p.nama).join(' & ')}`)

// ---------- share + live snapshot for viewers (names only) ----------
async function share() {
  const token = info.value.share || (await api('POST', S + '/share')).token; info.value.share = token
  const url = `${location.origin}/#/v/${token}`
  try { await navigator.clipboard.writeText(url); msg.value = 'View-only link copied: ' + url } catch { msg.value = 'View-only link: ' + url }
}
async function unshare() { await api('DELETE', S + '/share'); info.value.share = '' }
let liveT
watch(() => matches.value.map(m => [m.st, m.t1.map(p => p.nama), m.t2.map(p => p.nama)]), v => {
  clearTimeout(liveT)
  liveT = setTimeout(() => api('PUT', S + '/live', {
    playing: v.filter(x => x[0] === 'play').map(x => [x[1], x[2]]), next: v.filter(x => x[0] === 'queue').map(x => [x[1], x[2]]),
  }).catch(() => {}), 400)
}, { immediate: true })

// ---------- teams ----------
const inTeam = id => teams.value.some(t => t.p1 === id || t.p2 === id)
const teamIn = computed(() => rows.value.filter(r => !inTeam(r.id)))
const tname = t => `${nm(t.p1)} & ${nm(t.p2)}`
async function addTeam() {
  const { p1, p2 } = tp.value
  if (!p1 || !p2 || p1 === p2) { msg.value = 'Pick two different players.'; return }
  msg.value = ''; await api('POST', S + '/teams', tp.value); tp.value = { ...tp.value, p1: '', p2: '' }; teams.value = await api('GET', S + '/teams')
}
async function delTeam(t) { await api('DELETE', `${S}/teams/${t.id}`); teams.value = teams.value.filter(x => x !== t) }

const optsFor = ex => teamIn.value.filter(r => r.id !== ex).map(r => ({ id: r.id, label: r.nama, sub: r.level || '-' }))
const manOpts = i => manOptions(i).map(r => ({ id: r.id, label: r.nama, sub: (r.level || '-') + (r.st === 'queue' ? ' · up next' : '') }))
// ---------- leaderboard ----------
const sortBy = k => (a, b) => b[k] - a[k] || b.diff - a.diff || a.games - b.games
const board = computed(() => {
  if (mode.value === 'team') {
    const t = teams.value.map(x => ({ ...x, nama: tname(x), games: 0, wins: 0, pts: 0, diff: 0 }))
    for (const d of done.value) {
      const A = t.find(x => has(d.p.slice(0, 2), x)), B = t.find(x => has(d.p.slice(2), x)); if (!A || !B) continue
      A.games++; B.games++; A.pts += d.s1; B.pts += d.s2; A.diff += d.s1 - d.s2; B.diff += d.s2 - d.s1
      if (d.s1 > d.s2) A.wins++; else if (d.s2 > d.s1) B.wins++
    }
    return t.filter(x => x.games).sort(sortBy('wins'))
  }
  const t = {}
  for (const d of done.value) for (const [ids, f, a] of [[d.p.slice(0, 2), d.s1, d.s2], [d.p.slice(2), d.s2, d.s1]]) for (const id of ids) {
    const x = (t[id] ??= { nama: nm(id), games: 0, wins: 0, pts: 0, diff: 0 }); x.games++; x.wins += f > a; x.pts += f; x.diff += f - a
  }
  return Object.values(t).sort(sortBy(mode.value === 'americano' ? 'pts' : 'wins'))
})
const gt = computed(() => [1, 2].map(g => board.value.filter(x => x.grp === g).reduce((n, x) => n + x.wins, 0)))
const stat = r => (mode.value === 'americano' ? `${r.pts} pts` : `${r.wins}W ${r.games - r.wins}L  ${r.diff > 0 ? '+' : ''}${r.diff}`)
function draw(list) {
  const c = document.createElement('canvas'); c.width = 1080; c.height = 1350
  const x = c.getContext('2d'), F = 'system-ui,-apple-system,sans-serif', tm = mode.value === 'team'
  x.fillStyle = '#123D2B'; x.fillRect(0, 0, 1080, 1350)
  x.fillStyle = '#D9F344'; x.font = `800 64px ${F}`; x.fillText(info.value.name.toUpperCase().slice(0, 24), 70, 140)
  x.fillStyle = '#F6F7F2'; x.font = `400 34px ${F}`; x.fillText(`${MODES[mode.value]} · ${fmtDate(info.value.play_date)}`, 70, 200)
  let y0 = 250
  if (tm) { x.fillStyle = '#D9F344'; x.font = `800 44px ${F}`; x.fillText(`Group A  ${gt.value[0]}  –  ${gt.value[1]}  Group B`, 70, 270); y0 = 305 }
  list.slice(0, 10).forEach((r, i) => {
    const y = y0 + i * 96
    x.fillStyle = i < 3 ? '#D9F344' : '#1c5a3e'; x.beginPath(); x.roundRect(70, y, 940, 84, 22); x.fill()
    x.fillStyle = i < 3 ? '#123D2B' : '#F6F7F2'
    x.font = `800 40px ${F}`; x.fillText(String(i + 1), 105, y + 56)
    x.font = `700 ${tm ? 30 : 38}px ${F}`; x.fillText(r.nama, 185, y + 55)
    x.font = `500 30px ${F}`; x.textAlign = 'right'; x.fillText(stat(r), 985, y + 54); x.textAlign = 'left'
  })
  x.fillStyle = '#D9F344'; x.font = `500 30px ${F}`; x.fillText('Play together. Get better.', 70, 1315)
  return c
}
const preview = computed(() => (board.value.length ? draw(board.value).toDataURL() : ''))
const blob = () => new Promise(res => draw(board.value).toBlob(res, 'image/png'))
async function copyBoard() {
  const b = await blob()
  try { await navigator.clipboard.write([new ClipboardItem({ 'image/png': b })]); msg.value = 'Image copied. Paste it into Instagram.' }
  catch { downloadBoard(b); msg.value = 'The browser blocked copying (needs https), so the image was downloaded instead.' }
}
async function downloadBoard(b) { const a = document.createElement('a'); a.href = URL.createObjectURL(b || (await blob())); a.download = 'leaderboard.png'; a.click() }
</script>

<template>
  <div class="op">
    <nav>
      <button :class="{ on: tab === 'play' }" @click="tab = 'play'">Play</button>
      <button :class="{ on: tab === 'board' }" @click="tab = 'board'">Leaderboard</button>
    </nav>
    <div class="acts back">
      <button class="btn sm" @click="$emit('back')">← All games</button>
      <button class="btn sm" @click="share">{{ info.share ? 'Copy share link' : 'Share (view only)' }}</button>
      <button v-if="info.share" class="btn ghost sm" @click="unshare">Stop sharing</button>
    </div>
    <p v-if="msg" class="msg">{{ msg }}</p>

    <template v-if="tab === 'play'">
      <header class="hero">
        <div><h1><input class="title-input" v-model="info.name" maxlength="100" aria-label="Game name" @change="rename" /></h1><p class="tag"><span class="mchip">{{ MODES[mode] }}</span>{{ info.play_date && fmtDate(info.play_date) }}</p></div>
        <div class="stats">
          <div><b>{{ rows.length }}</b><span>Registered</span></div><div><b>{{ paid }}</b><span>Paid</span></div>
          <div><b>{{ rows.length - paid }}</b><span>Pending</span></div><div><b>{{ freeRows.length }}</b><span>Waiting</span></div>
        </div>
      </header>

      <details v-if="mode === 'team'" class="card" open>
        <summary>Teams ({{ teams.length }})</summary>
        <p v-if="!teamIn.length" class="empty">Check in players first (search box under Players), then build teams from them.</p>
        <div class="manual">
          <PlayerPick v-model="tp.p1" :options="optsFor(tp.p2)" placeholder="Player 1" />
          <PlayerPick v-model="tp.p2" :options="optsFor(tp.p1)" placeholder="Player 2" />
          <select v-model.number="tp.grp"><option :value="1">Group A</option><option :value="2">Group B</option></select>
          <select v-model="tp.category"><option>Mixed</option><option>Women</option><option>Men</option></select>
        </div>
        <div class="acts"><button class="btn primary" @click="addTeam">Add team</button></div>
        <div class="teamlist">
          <div v-for="g in [1, 2]" :key="g"><h4>Group {{ g === 1 ? 'A' : 'B' }}</h4>
            <ul><li v-for="t in teams.filter(t => t.grp === g)" :key="t.id"><span>{{ tname(t) }} <small>{{ t.category }}</small></span><button class="btn ghost sm" @click="delTeam(t)">×</button></li></ul>
          </div>
        </div>
      </details>

      <section class="card">
        <div class="bar">
          <h2>Matches</h2>
          <div class="stepper">Courts
            <button class="btn sm" @click="setCourts(courts - 1)" aria-label="Fewer courts">−</button><b>{{ courts }}</b>
            <button class="btn sm" @click="setCourts(courts + 1)" aria-label="More courts">+</button>
          </div>
          <span class="chip">{{ playing.length }} playing · {{ Math.max(0, slots) }} free</span>
          <button class="btn" @click="generate">Generate match</button>
          <button class="btn primary" @click="generateRound">Generate round</button>
        </div>
        <p v-if="slots < 0" class="msg">Courts lowered: current matches keep playing, and no new match can start until a court frees up.</p>
        <p v-if="!matches.length" class="empty">No matches yet. Generate some{{ mode === 'team' ? ' (add teams first)' : ', or create one manually below' }}.</p>
        <div class="matches">
          <article v-for="m in ordered" :key="m.id" class="match" :class="{ queued: m.st === 'queue' }">
            <h3>{{ m.st === 'play' ? 'Playing' : 'Up next' }}</h3>
            <div class="teams">
              <template v-for="(side, si) in ['t1', 't2']" :key="side">
                <i v-if="si">vs</i>
                <div>
                  <template v-for="(p, i) in m[side]" :key="p.id">
                    <select v-if="editable(m)" class="pick" :value="p.id" @change="swap(m, side, i, $event.target.value)">
                      <option v-for="r in swapOptions" :key="r.id" :value="r.id">{{ r.nama }} ({{ r.level || '-' }})</option>
                    </select>
                    <span v-else>{{ p.nama }} <small>{{ p.level || '-' }}</small></span>
                  </template>
                </div>
              </template>
            </div>
            <div v-if="m.st === 'play'" class="scores">
              <input type="number" min="0" v-model.number="m.s1" aria-label="Team 1 score" /><em>{{ mode === 'americano' ? 'pts' : 'to 30' }}</em>
              <input type="number" min="0" v-model.number="m.s2" aria-label="Team 2 score" />
            </div>
            <div class="acts">
              <button v-if="m.st === 'play'" class="btn primary" :disabled="!valid(m)" @click="finish(m)">Finish</button>
              <button v-else class="btn primary" :disabled="slots <= 0" @click="start(m)">Play now</button>
              <button class="btn ghost" @click="copy(m)">Copy</button>
              <button class="btn ghost" @click="release(m)">{{ m.st === 'play' ? 'Cancel' : 'Remove' }}</button>
            </div>
          </article>
        </div>
      </section>

      <details v-if="mode !== 'team'" class="card">
        <summary>Create a match manually</summary>
        <p class="empty">Players already in an "Up next" match can be picked too. Their suggested match is removed.</p>
        <div class="manual">
          <PlayerPick v-for="(_, i) in man" :key="i" v-model="man[i]" :options="manOpts(i)" :placeholder="(i < 2 ? 'Team 1' : 'Team 2') + ' · player ' + ((i % 2) + 1)" />
        </div>
        <div class="acts"><button class="btn" @click="manual(false)">Add to up next</button><button class="btn primary" :disabled="slots <= 0" @click="manual(true)">Play now</button></div>
      </details>

      <section class="card">
        <div class="bar">
          <h2>Players</h2>
          <div class="combo">
            <input v-model="q" placeholder="Search or add player…" autocomplete="off" @focus="open = true" @blur="open = false" @keydown="onKey" />
            <template v-if="canNew">
              <select v-model="newLevel" aria-label="Level for new player"><option value="">New: no level</option><option v-for="l in LEVELS" :key="l">{{ l }}</option></select>
              <button class="btn primary" @click="addNew">+ Add "{{ q.trim() }}"{{ mode === 'regular' ? '' : ' (this game only)' }}</button>
            </template>
            <ul v-if="open && q.trim() && found.length" class="list">
              <li v-for="(p, i) in found" :key="p.id">
                <button type="button" :class="{ on: i === act, in: p.in }" @mouseenter="act = i" @mousedown.prevent="!p.in && checkIn(p.id)">
                  {{ p.nama }}<small>{{ p.initial_level || '-' }}</small><small v-if="p.in">✓ already in</small>
                </button>
              </li>
            </ul>
          </div>
        </div>
        <table class="tbl play">
          <thead><tr><th>No</th><th>Nama</th><th>Jumlah Main</th><th>Level</th><th>Bayar</th><th></th></tr></thead>
          <tbody>
            <tr v-for="(r, i) in rows" :key="r.id" :class="{ playing: r.st === 'play' }">
              <td data-l="No">{{ i + 1 }}</td>
              <td data-l="Nama">{{ r.nama }}<em v-if="r.st"> · {{ r.st === 'play' ? 'Playing' : 'Up next' }}</em></td>
              <td data-l="Jumlah Main">{{ r.games }}</td>
              <td data-l="Level"><select v-model="r.level" @change="setLevel(r)"><option value="">-</option><option v-for="l in LEVELS" :key="l">{{ l }}</option></select></td>
              <td data-l="Bayar"><select v-model="r.payment" :class="{ paid: r.payment !== 'belum' }" @change="setPay(r)">
                <option value="qris">QRIS</option><option value="cash">Cash</option><option value="belum">Belum bayar</option></select></td>
              <td><button class="btn ghost sm" :disabled="r.st !== null" @click="removeRow(r)">Remove</button></td>
            </tr>
          </tbody>
        </table>
        <p v-if="!rows.length" class="empty">Nobody has checked in yet.</p>
      </section>
    </template>

    <section v-else class="card">
      <div class="bar">
        <h2>Leaderboard</h2>
        <button class="btn" :disabled="!board.length" @click="downloadBoard()">Download</button>
        <button class="btn primary" :disabled="!board.length" @click="copyBoard">Copy image</button>
      </div>
      <p v-if="!board.length" class="empty">No finished matches yet.
        {{ mode === 'americano' ? 'Ranked by total points scored.' : mode === 'team' ? 'Teams ranked by wins, plus the Group A vs Group B total.' : 'Ranked by wins, then point difference.' }}</p>
      <div v-else class="board"><img :src="preview" alt="Leaderboard preview" /></div>
    </section>
  </div>
</template>
