export const LEVELS = ['A', 'B+', 'B', 'B-', 'C', 'N']
export const MODES = { regular: 'Regular open play', team: 'Team group', americano: 'Americano' }
export const api = async (m, u, b) => {
  const r = await fetch('/api' + u, { method: m, headers: { 'Content-Type': 'application/json' }, body: b ? JSON.stringify(b) : undefined })
  if (r.status === 401) dispatchEvent(new Event('unauth'))
  if (!r.ok) throw new Error(await r.text())
  return r.status === 204 ? null : r.json()
}
export const fmtDate = d => new Date(d + 'T12:00').toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
