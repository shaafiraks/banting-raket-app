<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { MODES, fmtDate } from './api'
const props = defineProps({ token: String })
const d = ref(null), err = ref(''), tab = ref('matches'), now = ref(Date.now()); let timer, t2
async function load() {
  try { const r = await fetch('/api/public/' + props.token); if (!r.ok) throw new Error('This link is not available (anymore).'); d.value = await r.json(); err.value = '' }
  catch (e) { err.value = e.message }
}
onMounted(() => { load(); timer = setInterval(load, 8000); t2 = setInterval(() => (now.value = Date.now()), 20000) }) // refreshes by itself
onUnmounted(() => { clearInterval(timer); clearInterval(t2) })
const live = computed(() => d.value?.live || {})
const playing = computed(() => (live.value.playing || []).filter(m => m && m.t1))
const next = computed(() => (live.value.next || []).filter(m => m && m.t1))
const avg = computed(() => live.value.avg || 20), courts = computed(() => live.value.courts || 1)
const results = computed(() => (d.value?.results || []).map((r, i) => ({ ...r, n: i + 1 })).reverse())
const mins = m => (m.at ? Math.max(0, Math.floor((now.value - m.at) / 60000)) : 0)
const lead = m => (m.s1 > m.s2 ? 1 : m.s2 > m.s1 ? 2 : 0)
const est = i => { const free = courts.value - playing.value.length; return free >= i + 1 ? 0 : Math.ceil((i + 1 - Math.max(0, free)) / courts.value) * avg.value }
const ord = n => ['1st', '2nd', '3rd'][n - 1] || n + 'th'
const win = (r, side) => (side === 1 ? r.s1 > r.s2 : r.s2 > r.s1)
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
const gt = computed(() => { const rs = d.value?.results || []; return [rs.filter(r => r.s1 > r.s2).length, rs.filter(r => r.s2 > r.s1).length] }) // matches won by each group
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
      <section v-if="d.mode === 'team'" class="scorecard">
        <div class="side" :class="{ ahead: gt[0] > gt[1] }"><span class="gn">{{ d.group_a || 'A' }}</span><b>{{ gt[0] }}</b></div>
        <i>vs</i>
        <div class="side" :class="{ ahead: gt[1] > gt[0] }"><span class="gn">{{ d.group_b || 'B' }}</span><b>{{ gt[1] }}</b></div>
      </section>
      <template v-if="tab === 'matches'">
        <p v-if="!playing.length && !next.length && !results.length" class="empty">No matches yet. This page updates by itself.</p>

        <template v-if="playing.length">
          <h3 class="sec">Currently on court <span class="pill">{{ playing.length }} live</span></h3>
          <article v-for="(m, i) in playing" :key="'p' + i" class="pcard">
            <header class="two"><span class="badge play">LIVE</span><span>{{ mins(m) }}m played</span></header>
            <div class="prow" :class="{ win: lead(m) === 1 }"><div><span v-for="(n, j) in m.t1" :key="j">{{ n }}</span></div><div class="end"><em v-if="lead(m) === 1" class="lead">LEAD</em><b>{{ m.s1 }}</b></div></div>
            <div class="prow" :class="{ win: lead(m) === 2 }"><div><span v-for="(n, j) in m.t2" :key="j">{{ n }}</span></div><div class="end"><em v-if="lead(m) === 2" class="lead">LEAD</em><b>{{ m.s2 }}</b></div></div>
            <footer>{{ lead(m) ? (lead(m) === 1 ? m.t1 : m.t2).join(' & ') + ' lead by ' + Math.abs(m.s1 - m.s2) : 'All square' }}</footer>
          </article>
        </template>

        <template v-if="next.length">
          <h3 class="sec">Upcoming queue <span class="pill">Next in line</span></h3>
          <p class="hint">Please stay court-ready</p>
          <article v-for="(m, i) in next" :key="'n' + i" class="pcard" :class="{ hl: i === 0 }">
            <header><span class="badge next">UP NEXT · {{ i + 1 }}</span><small v-if="i === 0" class="warm">Warm up now · {{ est(0) ? 'next free court in ~' + est(0) + ' mins' : 'a court is free' }}</small></header>
            <div class="prow"><div><span v-for="(n, j) in m.t1" :key="j">{{ n }}</span></div><em class="ab">Team A</em></div>
            <div class="prow"><div><span v-for="(n, j) in m.t2" :key="j">{{ n }}</span></div><em class="ab">Team B</em></div>
            <footer v-if="i">Queue position: {{ ord(i + 1) }} · {{ est(i) ? 'Est. ~' + est(i) + ' mins' : 'a court is free' }}</footer>
          </article>
        </template>

        <template v-if="results.length">
          <h3 class="sec">Results</h3>
          <article v-for="r in results" :key="r.n" class="pcard">
            <header><span class="badge">MATCH {{ r.n }}</span></header>
            <div class="prow" :class="win(r, 1) ? 'win' : 'lose'"><div><span v-for="(n, j) in r.t1" :key="j">{{ n }}</span></div><div class="end"><em v-if="win(r, 1)" class="lead">👑 WINNER</em><b>{{ r.s1 }}</b></div></div>
            <div class="prow" :class="win(r, 2) ? 'win' : 'lose'"><div><span v-for="(n, j) in r.t2" :key="j">{{ n }}</span></div><div class="end"><em v-if="win(r, 2)" class="lead">👑 WINNER</em><b>{{ r.s2 }}</b></div></div>
            <footer v-if="r.min">Finished in {{ r.min }} mins · Match ended</footer>
          </article>
        </template>
      </template>

      <section v-else class="card">
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
