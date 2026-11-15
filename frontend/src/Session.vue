<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { api, LEVELS, MODES, fmtDate } from './api'
import PlayerPick from './PlayerPick.vue'
const props = defineProps({ sid: Number }); defineEmits(['back'])
const S = `/sessions/${props.sid}`
const LV = { A: 5, 'B+': 4, B: 3, 'B-': 2, C: 1, N: 0 } // unknown level = 2
const tab = ref('play'), info = ref({ name: '', play_date: '', mode: 'regular' }), master = ref([]), rows = ref([]), matches = ref([]), done = ref([]), teams = ref([])
const courts = ref(3), msg = ref(''), toast = ref(''), q = ref(''), open = ref(false), act = ref(0), newLevel = ref(''), man = ref(['', '', '', ''])
const tp = ref({ p1: '', p2: '', grp: 1, category: 'Mixed' })
let uid = 0
const idx = ref(0), sw = ref(null), durs = ref([]), lbStyle = ref('solid'), now = ref(Date.now())
const mode = computed(() => info.value.mode)
const say = t => { toast.value = t; setTimeout(() => (toast.value = ''), 2200) }

async function load() {
  master.value = await api('GET', '/players')
  const s = await api('GET', S); info.value = s; courts.value = s.courts
  const old = new Map(rows.value.map(r => [r.id, r])) // keep planned game counts across reloads
  rows.value = s.players.map(p => old.get(p.id) || { ...p, last: 0 })
  done.value = await api('GET', S + '/matches'); teams.value = await api('GET', S + '/teams')
}
onMounted(() => load().catch(e => (msg.value = e.message)))

const nm = id => rows.value.find(r => r.id === id)?.nama || id
const nextId = () => String(Math.max(0, ...master.value.map(p => parseInt(p.id) || 0)) + 1).padStart(4, '0')
const found = computed(() => master.value.filter(p => p.nama.toLowerCase().includes(q.value.trim().toLowerCase())).slice(0, 8).map(p => ({ ...p, in: rows.value.some(r => r.id === p.id) })))
const canNew = computed(() => q.value.trim() && !master.value.some(p => p.nama.toLowerCase() === q.value.trim().toLowerCase()))
const paid = computed(() => rows.value.filter(r => r.payment !== 'belum').length)
const inM = m => [...m.t1, ...m.t2]
const stMap = computed(() => { const o = new Map(); for (const x of matches.value) for (const p of inM(x)) if (x.st === 'play' || !o.has(p.id)) o.set(p.id, x.st); return o })
const st = r => stMap.value.get(r.id) || null // 'play' | 'queue' | null
const waiting = computed(() => rows.value.filter(r => !st(r)).length)
const playing = computed(() => matches.value.filter(m => m.st === 'play'))
const slots = computed(() => courts.value - playing.value.length)
const ordered = computed(() => [...matches.value].sort((a, b) => (a.st === 'play' ? 0 : 1) - (b.st === 'play' ? 0 : 1) || a.id - b.id))
const ongoing = computed(() => matches.value.filter(m => m.st === 'play'))
const upcoming = computed(() => matches.value.filter(m => m.st === 'queue').sort((a, b) => a.id - b.id))
const history = computed(() => done.value.map((d, i) => ({ ...d, n: i + 1 })).reverse())
const panes = computed(() => [{ k: 'on', label: 'Ongoing', n: ongoing.value.length }, { k: 'up', label: 'Upcoming', n: upcoming.value.length }, { k: 'hi', label: 'History', n: history.value.length }])
const sub = r => `${r.level || '-'} - ${r.games}x`
const lab = r => `${r.nama} (${sub(r)})`
const editable = m => m.st === 'queue' && mode.value !== 'team'
const opts = (m, p) => rows.value.filter(r => r.id === p.id || !inM(m).some(x => x.id === r.id))
// a queued match can't start while one of its players is still on court
const blocked = m => (m.st === 'queue' ? inM(m).filter(p => matches.value.some(x => x.st === 'play' && inM(x).some(y => y.id === p.id))) : [])
const canStart = m => slots.value > 0 && !blocked(m).length

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
// "Jumlah main" counts a game from the moment it is generated (removing the match takes it back).
const val = r => LV[r.level] ?? 2
const pk = (a, b) => [a.id, b.id].sort().join('|')
const partnered = computed(() => {
  const o = new Map(), add = t => { const k = t.slice().sort().join('|'); o.set(k, (o.get(k) || 0) + 1) }
  for (const d of done.value) { add(d.p.slice(0, 2)); add(d.p.slice(2)) }
  for (const x of matches.value) { add(x.t1.map(p => p.id)); add(x.t2.map(p => p.id)) }
  return o
})
function mk(t1, t2, extra = {}) {
  const m = { id: ++uid, st: 'queue', t1, t2, s1: null, s2: null, ...extra }
  inM(m).forEach(p => { p.games++; p.last = Date.now() })
  matches.value.push(m); return matches.value[matches.value.length - 1] // reactive version
}
function build() {
  const am = mode.value === 'americano'
  const s = rows.value.slice().sort((a, b) => a.games - b.games || a.last - b.last || Math.random() - 0.5)
  if (s.length < 4) return null
  const a = s[0]; let rest = s.slice(1)
  if (!am) for (let g = 1; g <= 5; g++) { rest = s.slice(1).filter(r => Math.abs(val(r) - val(a)) <= g); if (rest.length >= 3) break }
  const [p, q2, r, t] = [a, ...rest.slice(0, 3)].sort((x, y) => val(y) - val(x))
  const sum = x => x.reduce((n, y) => n + val(y), 0)
  const cost = o => (am ? 0 : Math.abs(sum(o[0]) - sum(o[1]))) + 3 * o.reduce((n, x) => n + (partnered.value.get(pk(...x)) || 0), 0)
  const best = [[[p, t], [q2, r]], [[p, q2], [r, t]], [[p, r], [q2, t]]].sort((x, y) => cost(x) - cost(y))[0]
  return mk(best[0], best[1])
}
// team group: A team vs B team, fewest planned games, no rematch, same category preferred
const has = (x, t) => x.includes(t.p1) && x.includes(t.p2)
const tgames = t => done.value.filter(d => has(d.p.slice(0, 2), t) || has(d.p.slice(2), t)).length + matches.value.filter(m => m.a?.id === t.id || m.b?.id === t.id).length
const met = (a, b) => done.value.some(d => { const x = d.p.slice(0, 2), y = d.p.slice(2); return (has(x, a) && has(y, b)) || (has(x, b) && has(y, a)) }) ||
  matches.value.some(m => (m.a?.id === a.id && m.b?.id === b.id) || (m.a?.id === b.id && m.b?.id === a.id))
function buildTeam() {
  let best = null, bc = 1e9
  for (const a of teams.value.filter(t => t.grp === 1)) for (const b of teams.value.filter(t => t.grp === 2)) {
    if ([a.p1, a.p2].some(id => id === b.p1 || id === b.p2)) continue // same person can't be on both sides
    const c = tgames(a) + tgames(b) + (met(a, b) ? 5 : 0) + (a.category !== b.category ? 3 : 0) + Math.random() * 0.5
    if (c < bc) { bc = c; best = [a, b] }
  }
  if (!best) return null
  const R = id => rows.value.find(r => r.id === id)
  return mk([R(best[0].p1), R(best[0].p2)], [R(best[1].p1), R(best[1].p2)], { a: best[0], b: best[1] })
}
const gen = () => (mode.value === 'team' ? buildTeam() : build())
const noneMsg = () => (mode.value === 'team' ? 'Need at least one team in each group.' : 'Need at least 4 checked-in players.')
const generate = () => { msg.value = ''; if (!gen()) msg.value = noneMsg() }
function generateRound() { // one more game for everybody: players / 4 matches (or one per team pair)
  msg.value = ''
  const n = mode.value === 'team' ? Math.min(teams.value.filter(t => t.grp === 1).length, teams.value.filter(t => t.grp === 2).length) : Math.floor(rows.value.length / 4)
  let made = 0; for (let i = 0; i < n; i++) { if (gen()) made++; else break }
  if (!made) msg.value = noneMsg()
}
const start = m => { if (canStart(m)) { m.st = 'play'; m.at = Date.now() } }
const drop = m => (matches.value = matches.value.filter(x => x !== m))
function release(m) { inM(m).forEach(p => (p.games = Math.max(0, p.games - 1))); drop(m) }
function swap(m, side, i, id) {
  const old = m[side][i], nu = rows.value.find(r => r.id === id)
  if (!nu || nu === old) return
  old.games = Math.max(0, old.games - 1); nu.games++; nu.last = Date.now(); m[side][i] = nu
}
const manOpts = i => rows.value.filter(r => !man.value.some((v, j) => j !== i && v === r.id)).map(r => ({ id: r.id, label: r.nama, sub: sub(r) + (st(r) === 'play' ? ' · playing' : '') }))
function manual(playNow) {
  const [a, b, c, d] = man.value.map(id => rows.value.find(r => r.id === id))
  if (![a, b, c, d].every(Boolean)) { msg.value = 'Pick 4 players first.'; return }
  const m = mk([a, b], [c, d]); man.value = ['', '', '', '']; msg.value = ''
  if (playNow) { if (canStart(m)) start(m); else msg.value = 'Added to Up next: no free court, or a player is still playing.' }
}
const valid = m => Number.isFinite(m.s1) && Number.isFinite(m.s2) && (mode.value === 'americano' || (m.s1 !== m.s2 && Math.max(m.s1, m.s2) >= 30))
async function finish(m) {
  const ids = inM(m).map(p => p.id), min = m.at ? Math.max(1, Math.round((Date.now() - m.at) / 60000)) : 0
  await api('POST', S + '/matches', { players: ids, score1: m.s1, score2: m.s2, minutes: min })
  if (min) durs.value.push(min)
  done.value.push({ p: ids, s1: m.s1, s2: m.s2, min }); drop(m) // games were already counted when generated
}
const copy = m => navigator.clipboard.writeText(`${m.t1.map(p => p.nama).join(' & ')}\nvs\n${m.t2.map(p => p.nama).join(' & ')}`).then(() => say('Copied'), () => say('Copy failed'))

// ---------- share + live snapshot for viewers (names only) ----------
async function share() {
  const token = info.value.share || (await api('POST', S + '/share')).token; info.value.share = token
  const url = `${location.origin}/#/v/${token}`
  try { await navigator.clipboard.writeText(url); say('View-only link copied') } catch { msg.value = 'View-only link: ' + url }
}
async function unshare() { await api('DELETE', S + '/share'); info.value.share = '' }
let liveT
const snap = () => ({ // what viewers see: names, live points, start time. No levels or payments.
  playing: ongoing.value.map(m => ({ t1: m.t1.map(p => p.nama), t2: m.t2.map(p => p.nama), s1: Number.isFinite(m.s1) ? m.s1 : 0, s2: Number.isFinite(m.s2) ? m.s2 : 0, at: m.at || 0 })),
  next: upcoming.value.map(m => ({ t1: m.t1.map(p => p.nama), t2: m.t2.map(p => p.nama) })),
  avg: durs.value.length ? Math.round(durs.value.reduce((x, y) => x + y, 0) / durs.value.length) : 20, courts: courts.value,
})
watch(snap, v => { clearTimeout(liveT); liveT = setTimeout(() => api('PUT', S + '/live', v).catch(() => {}), 400) }, { immediate: true })

// ---------- swipe carousel: Ongoing | Upcoming | History ----------
function go(i) { idx.value = i; sw.value?.scrollTo({ left: i * sw.value.clientWidth, behavior: 'smooth' }) }
function onSwipe() { const i = Math.round(sw.value.scrollLeft / sw.value.clientWidth); if (i !== idx.value) idx.value = i }
function fit() { const el = sw.value?.children[idx.value]; if (el) sw.value.style.height = el.offsetHeight + 'px' }
watch(sw, el => { if (!el) return; idx.value = 0; const ro = new ResizeObserver(fit); [...el.children].forEach(c => ro.observe(c)); fit() })
watch(idx, () => nextTick(fit))
let tick; onMounted(() => (tick = setInterval(() => (now.value = Date.now()), 30000))); onUnmounted(() => clearInterval(tick))

// ---------- teams (a player can be in several teams) ----------
const tname = t => `${nm(t.p1)} & ${nm(t.p2)}`
async function addTeam() {
  const { p1, p2 } = tp.value
  if (!p1 || !p2 || p1 === p2) { msg.value = 'Pick two different players.'; return }
  msg.value = ''; await api('POST', S + '/teams', tp.value); tp.value = { ...tp.value, p1: '', p2: '' }; teams.value = await api('GET', S + '/teams')
}
async function delTeam(t) { await api('DELETE', `${S}/teams/${t.id}`); teams.value = teams.value.filter(x => x !== t) }
const optsFor = ex => rows.value.filter(r => r.id !== ex).map(r => ({ id: r.id, label: r.nama, sub: sub(r) }))

// ---------- leaderboard ----------
const sortBy = k => (a, b) => b[k] - a[k] || b.diff - a.diff || a.games - b.games
const board = computed(() => {
  if (mode.value === 'team') {
    const t = teams.value.map(x => ({ ...x, nama: tname(x), games: 0, wins: 0, pts: 0, diff: 0 }))
    for (const d of done.value) {
      const A = t.find(x => x.grp === 1 && has(d.p.slice(0, 2), x)) || t.find(x => has(d.p.slice(0, 2), x))
      const B = t.find(x => x.grp === 2 && has(d.p.slice(2), x)) || t.find(x => has(d.p.slice(2), x)); if (!A || !B) continue
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
  const sty = lbStyle.value // solid | rounded (30px corners) | transparent
  if (sty !== 'transparent') { x.fillStyle = '#123D2B'; x.beginPath(); x.roundRect(0, 0, 1080, 1350, sty === 'rounded' ? 30 : 0); x.fill() }
  else { x.shadowColor = '#000000b0'; x.shadowBlur = 14 }
  x.fillStyle = '#D9F344'; x.font = `800 64px ${F}`; x.fillText(info.value.name.toUpperCase().slice(0, 24), 70, 140)
  x.fillStyle = '#F6F7F2'; x.font = `400 34px ${F}`; x.fillText(`${MODES[mode.value]} · ${fmtDate(info.value.play_date)}`, 70, 200)
  let y0 = 250
  if (tm) { x.fillStyle = '#D9F344'; x.font = `800 44px ${F}`; x.fillText(`Group A  ${gt.value[0]}  –  ${gt.value[1]}  Group B`, 70, 270); y0 = 305 }
  list.slice(0, 10).forEach((r, i) => {
    const y = y0 + i * 96
    x.fillStyle = i < 3 ? '#D9F344' : sty === 'transparent' ? '#123D2Bdd' : '#1c5a3e'; x.beginPath(); x.roundRect(70, y, 940, 84, 22); x.fill()
    x.fillStyle = i < 3 ? '#123D2B' : '#F6F7F2'
    x.font = `800 40px ${F}`; x.fillText(String(i + 1), 105, y + 56)
    x.font = `700 ${tm ? 30 : 38}px ${F}`; x.fillText(r.nama, 185, y + 55)
    x.font = `500 30px ${F}`; x.textAlign = 'right'; x.fillText(stat(r), 985, y + 54); x.textAlign = 'left'
  })
  x.fillStyle = '#D9F344'; x.font = `500 30px ${F}`; x.fillText('Play together. Get better.', 70, 1315)
  return c
}
const preview = computed(() => (board.value.length ? draw(board.value).toDataURL() : ''))
const makeBlob = () => new Promise((res, rej) => draw(board.value).toBlob(b => (b ? res(b) : rej(new Error('no image'))), 'image/png'))
// Safari only allows the clipboard write inside the tap itself, so it is called right away with a promise for the image
function copyBoard() {
  const png = makeBlob()
  navigator.clipboard.write([new ClipboardItem({ 'image/png': png })]).then(() => say('Copied to clipboard'), async () => {
    try { const f = new File([await png], 'leaderboard.png', { type: 'image/png' }); if (navigator.canShare?.({ files: [f] })) { await navigator.share({ files: [f] }); return } } catch { /* fall through */ }
    downloadBoard(await png); say('Could not copy, saved as an image instead')
  })
}
async function downloadBoard(b) { const a = document.createElement('a'); a.href = URL.createObjectURL(b || (await makeBlob())); a.download = 'leaderboard.png'; a.click() }
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
          <div><b>{{ rows.length - paid }}</b><span>Pending</span></div><div><b>{{ waiting }}</b><span>Not in a match</span></div>
        </div>
      </header>

      <details v-if="mode === 'team'" class="card" open>
        <summary>Teams ({{ teams.length }})</summary>
        <p class="empty">A player can be in more than one team, and the same pair can be added more than once.</p>
        <p v-if="!rows.length" class="empty">Check in players first (search box under Players), then build teams from them.</p>
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
          <span class="chip">{{ playing.length }} playing · {{ Math.max(0, slots) }} free · {{ matches.length - playing.length }} next</span>
          <button class="btn" @click="generate">Generate match</button>
          <button class="btn primary" @click="generateRound">Generate round</button>
        </div>
        <p v-if="slots < 0" class="msg">Courts lowered: current matches keep playing, and no new match can start until a court frees up.</p>
        <p v-if="!matches.length" class="empty">No matches yet. Generate as many as you like, even more than the courts, so players can get ready.</p>
        <div class="seg">
          <button v-for="(t, i) in panes" :key="t.k" :class="{ on: idx === i }" @click="go(i)">{{ t.label }}<b>{{ t.n }}</b></button>
        </div>
        <div ref="sw" class="swipe" @scroll.passive="onSwipe">
          <div v-for="(list, pi) in [ongoing, upcoming]" :key="pi" class="pane">
            <p v-if="!list.length" class="empty">{{ pi ? 'Nothing up next. Generate some matches above.' : 'Nothing on court. Press Play now on an Up next match.' }}</p>
            <article v-for="m in list" :key="m.id" class="match" :class="{ queued: m.st === 'queue' }">
              <h3>{{ m.st === 'play' ? 'Playing' + (m.at ? ' · ' + Math.floor((now - m.at) / 60000) + 'm' : '') : 'Up next' }}</h3>
              <div class="teams">
                <template v-for="(side, si) in ['t1', 't2']" :key="side">
                  <i v-if="si">vs</i>
                  <div>
                    <template v-for="(p, i) in m[side]" :key="p.id">
                      <select v-if="editable(m)" class="pick" :value="p.id" @change="swap(m, side, i, $event.target.value)">
                        <option v-for="r in opts(m, p)" :key="r.id" :value="r.id">{{ lab(r) }}</option>
                      </select>
                      <span v-else>{{ p.nama }} <small>{{ sub(p) }}</small></span>
                    </template>
                  </div>
                </template>
              </div>
              <div v-if="m.st === 'play'" class="scores">
                <input type="number" min="0" v-model.number="m.s1" aria-label="Team 1 score" /><em>{{ mode === 'americano' ? 'pts' : 'to 30' }}</em>
                <input type="number" min="0" v-model.number="m.s2" aria-label="Team 2 score" />
              </div>
              <p v-if="blocked(m).length" class="wait">Waiting for {{ blocked(m).map(p => p.nama).join(', ') }} to finish</p>
              <div class="acts">
                <button v-if="m.st === 'play'" class="btn primary" :disabled="!valid(m)" @click="finish(m)">Finish</button>
                <button v-else class="btn primary" :disabled="!canStart(m)" @click="start(m)">Play now</button>
                <button class="btn ghost" @click="copy(m)">Copy</button>
                <button class="btn ghost" @click="release(m)">{{ m.st === 'play' ? 'Cancel' : 'Remove' }}</button>
              </div>
            </article>
          </div>
          <div class="pane">
            <p v-if="!history.length" class="empty">No finished matches yet.</p>
            <article v-for="h in history" :key="h.n" class="pcard">
              <header><span class="badge">MATCH {{ h.n }}</span></header>
              <div class="prow" :class="h.s1 > h.s2 ? 'win' : 'lose'"><div><span v-for="id in h.p.slice(0, 2)" :key="id">{{ nm(id) }}</span></div><b>{{ h.s1 }}</b></div>
              <div class="prow" :class="h.s2 > h.s1 ? 'win' : 'lose'"><div><span v-for="id in h.p.slice(2)" :key="id">{{ nm(id) }}</span></div><b>{{ h.s2 }}</b></div>
              <footer v-if="h.min">Finished in {{ h.min }} mins</footer>
            </article>
          </div>
        </div>
      </section>

      <details v-if="mode !== 'team'" class="card">
        <summary>Create a match manually</summary>
        <p class="empty">Pick any checked-in players, for example a late request.</p>
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
            <tr v-for="(r, i) in rows" :key="r.id" :class="{ playing: st(r) === 'play' }">
              <td data-l="No">{{ i + 1 }}</td>
              <td data-l="Nama">{{ r.nama }}<em v-if="st(r)"> · {{ st(r) === 'play' ? 'Playing' : 'Up next' }}</em></td>
              <td data-l="Jumlah Main">{{ r.games }}</td>
              <td data-l="Level"><select v-model="r.level" @change="setLevel(r)"><option value="">-</option><option v-for="l in LEVELS" :key="l">{{ l }}</option></select></td>
              <td data-l="Bayar"><select v-model="r.payment" :class="{ paid: r.payment !== 'belum' }" @change="setPay(r)">
                <option value="qris">QRIS</option><option value="cash">Cash</option><option value="belum">Belum bayar</option></select></td>
              <td><button class="btn ghost sm" :disabled="!!st(r)" @click="removeRow(r)">Remove</button></td>
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
      <div v-else class="board">
        <div class="seg" style="width:100%;max-width:480px">
          <button v-for="o in [['solid', 'Solid'], ['rounded', 'Rounded'], ['transparent', 'Transparent']]" :key="o[0]" :class="{ on: lbStyle === o[0] }" @click="lbStyle = o[0]">{{ o[1] }}</button>
        </div>
        <img :class="{ chk: lbStyle === 'transparent' }" :src="preview" alt="Leaderboard preview" />
      </div>
    </section>
    <div v-if="toast" class="toast" role="status">{{ toast }}</div>
  </div>
</template>
