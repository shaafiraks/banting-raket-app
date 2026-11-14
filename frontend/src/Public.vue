<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { MODES, fmtDate } from './api'
const props = defineProps({ token: String })
const d = ref(null), err = ref(''), tab = ref('matches'); let timer
async function load() {
  try { const r = await fetch('/api/public/' + props.token); if (!r.ok) throw new Error('This link is not available (anymore).'); d.value = await r.json(); err.value = '' }
  catch (e) { err.value = e.message }
}
onMounted(() => { load(); timer = setInterval(load, 15000) }) // refreshes by itself
onUnmounted(() => clearInterval(timer))
const live = computed(() => d.value?.live || {})
const results = computed(() => (d.value?.results || []).map((r, i) => ({ ...r, n: i + 1 })).reverse())
const cards = computed(() => [
  ...(live.value.playing || []).map(m => ({ k: 'play', badge: 'NOW PLAYING', t1: m[0], t2: m[1] })),
  ...(live.value.next || []).map((m, i) => ({ k: 'next', badge: `UP NEXT · ${i + 1}`, t1: m[0], t2: m[1] })),
  ...results.value.map(r => ({ ...r, k: 'done', badge: `MATCH ${r.n}` })),
])
const titles = { play: 'Now playing', next: 'Up next', done: 'Results' }
const cls = (c, side) => (c.k === 'done' ? ((side === 1 ? c.s1 > c.s2 : c.s2 > c.s1) ? 'win' : 'lose') : '')
const sortBy = k => (a, b) => b[k] - a[k] || b.diff - a.diff || a.games - b.games
const board = computed(() => {
  const rs = d.value?.results || [], mode = d.value?.mode, t = {}
  if (mode === 'team') {
    const key = a => [...a].sort().join('|')
    for (const x of d.value.teams || []) t[key(x.names)] = { nama: x.names.join(' & '), grp: x.grp, games: 0, wins: 0, pts: 0, diff: 0 }
    for (const r of rs) {
      const A = t[key(r.t1)], B = t[key(r.t2)]; if (!A || !B) continue
      A.games++; B.games++; A.pts += r.s1; B.pts += r.s2; A.diff += r.s1 - r.s2; B.diff += r.s2 - r.s1
      if (r.s1 > r.s2) A.wins++; else if (r.s2 > r.s1) B.wins++
    }
    return Object.values(t).filter(x => x.games).sort(sortBy('wins'))
  }
  for (const r of rs) for (const [ns, f, a] of [[r.t1, r.s1, r.s2], [r.t2, r.s2, r.s1]]) for (const n of ns) {
    const x = (t[n] ??= { nama: n, games: 0, wins: 0, pts: 0, diff: 0 }); x.games++; x.wins += f > a; x.pts += f; x.diff += f - a
  }
  return Object.values(t).sort(sortBy(mode === 'americano' ? 'pts' : 'wins'))
})
const gt = computed(() => [1, 2].map(g => board.value.filter(x => x.grp === g).reduce((n, x) => n + x.wins, 0)))
</script>
<template>
  <div class="op">
    <nav>
      <button :class="{ on: tab === 'matches' }" @click="tab = 'matches'">Matches</button>
      <button :class="{ on: tab === 'board' }" @click="tab = 'board'">Leaderboard</button>
    </nav>
    <p v-if="err" class="msg">{{ err }}</p>
    <template v-if="d">
      <header class="hero">
        <div><h1>{{ d.name }}</h1><p class="tag"><span class="mchip">{{ MODES[d.mode] }}</span>{{ fmtDate(d.play_date) }}</p></div>
      </header>
      <template v-if="tab === 'matches'">
        <p v-if="!cards.length" class="empty">No matches yet. This page updates by itself.</p>
        <template v-for="(c, i) in cards" :key="c.k + i">
          <h3 v-if="!i || cards[i - 1].k !== c.k" class="sec">{{ titles[c.k] }}</h3>
          <article class="pcard">
            <header><span class="badge" :class="c.k">{{ c.badge }}</span></header>
            <div class="prow" :class="cls(c, 1)"><div><span v-for="(n, j) in c.t1" :key="j">{{ n }}</span></div><b v-if="c.k === 'done'">{{ c.s1 }}</b></div>
            <div class="prow" :class="cls(c, 2)"><div><span v-for="(n, j) in c.t2" :key="j">{{ n }}</span></div><b v-if="c.k === 'done'">{{ c.s2 }}</b></div>
          </article>
        </template>
      </template>
      <section v-else class="card">
        <p v-if="d.mode === 'team'" class="grp">Group A {{ gt[0] }} – {{ gt[1] }} Group B</p>
        <p v-if="!board.length" class="empty">No finished matches yet.</p>
        <table v-else class="tbl">
          <thead><tr><th>#</th><th>{{ d.mode === 'team' ? 'Team' : 'Player' }}</th><th>{{ d.mode === 'americano' ? 'Pts' : 'W-L' }}</th><th>+/-</th></tr></thead>
          <tbody><tr v-for="(r, i) in board" :key="r.nama + i">
            <td>{{ i + 1 }}</td><td>{{ r.nama }}</td><td>{{ d.mode === 'americano' ? r.pts : `${r.wins}-${r.games - r.wins}` }}</td><td>{{ r.diff > 0 ? '+' : '' }}{{ r.diff }}</td>
          </tr></tbody>
        </table>
      </section>
    </template>
  </div>
</template>
