package main

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

//go:embed schema.sql seed.sql
var files embed.FS

var db *sql.DB

type Player struct {
	ID    string  `json:"id"`
	Nama  string  `json:"nama"`
	Level *string `json:"initial_level"`
}

func js(w http.ResponseWriter, v any) { w.Header().Set("Content-Type", "application/json"); json.NewEncoder(w).Encode(v) }
func bad(w http.ResponseWriter, err error) { log.Println(err); http.Error(w, err.Error(), 500) }
func s(p *string) string { if p == nil { return "" }; return *p }
func sidOf(r *http.Request) int { n, _ := strconv.Atoi(r.PathValue("sid")); return n }
func body[T any](r *http.Request) (T, error) { var v T; err := json.NewDecoder(r.Body).Decode(&v); return v, err }
func do(w http.ResponseWriter, q string, a ...any) {
	if _, err := db.Exec(q, a...); err != nil { bad(w, err); return }
	w.WriteHeader(204)
}
func rowsOf(q string, f func(*sql.Rows) any, a ...any) []any {
	out := []any{}
	rs, err := db.Query(q, a...)
	if err != nil { log.Println(err); return out }
	defer rs.Close()
	for rs.Next() { out = append(out, f(rs)) }
	return out
}

func auth(pw string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, p, ok := r.BasicAuth(); !ok || p != pw {
			w.Header().Set("WWW-Authenticate", `Basic realm="Banting Raket"`)
			http.Error(w, "Unauthorized", 401)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func rnd(n int) string { b := make([]byte, n); rand.Read(b); return hex.EncodeToString(b) }

// every /api route needs a login cookie, except login itself and the read-only public view
func guard(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/") {
			w.Header().Set("Cache-Control", "no-store") // never cache API answers
			w.Header().Add("Vary", "Cookie")
		}
		if strings.HasPrefix(p, "/api/") && p != "/api/login" && !strings.HasPrefix(p, "/api/public/") {
			c, err := r.Cookie("bt")
			var n int
			if err != nil || db.QueryRow(`SELECT 1 FROM auth_tokens WHERE token=$1 AND created_at > now() - interval '7 days'`, c.Value).Scan(&n) != nil {
				http.Error(w, "unauthorized", 401)
				return
			}
			db.Exec(`UPDATE auth_tokens SET last_seen=now() WHERE token=$1 AND last_seen < now() - interval '1 minute'`, c.Value)
		}
		h.ServeHTTP(w, r)
	})
}

func main() {
	var err error
	if db, err = sql.Open("pgx", os.Getenv("DATABASE_URL")); err != nil { log.Fatal(err) }
	sc, _ := files.ReadFile("schema.sql")
	if _, err = db.Exec(string(sc)); err != nil { log.Fatal(err) }
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&n)
	if n == 0 { sd, _ := files.ReadFile("seed.sql"); if _, err = db.Exec(string(sd)); err != nil { log.Fatal(err) } }

	var uc int
	db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&uc)
	if uc == 0 { // first admin comes from env
		u, pw := os.Getenv("ADMIN_USER"), os.Getenv("ADMIN_PASSWORD")
		if u == "" { u = "admin" }
		if pw == "" { pw = "admin123"; log.Println("WARNING: default admin password in use, set ADMIN_PASSWORD") }
		hs, _ := bcrypt.GenerateFromPassword([]byte(pw), 10)
		db.Exec(`INSERT INTO users(username,password_hash) VALUES($1,$2)`, u, string(hs))
	}

	m := http.NewServeMux()
	// ---- master players
	m.HandleFunc("GET /api/players", func(w http.ResponseWriter, r *http.Request) {
		js(w, rowsOf(`SELECT id,nama,initial_level FROM players ORDER BY nama`, func(rs *sql.Rows) any { var p Player; rs.Scan(&p.ID, &p.Nama, &p.Level); return p }))
	})
	m.HandleFunc("POST /api/players", func(w http.ResponseWriter, r *http.Request) {
		p, err := body[Player](r)
		if err != nil || p.ID == "" || p.Nama == "" { http.Error(w, "id and nama required", 400); return }
		do(w, `INSERT INTO players(id,nama,initial_level) VALUES($1,$2,NULLIF($3,''))`, p.ID, p.Nama, s(p.Level))
	})
	m.HandleFunc("PUT /api/players/{id}", func(w http.ResponseWriter, r *http.Request) {
		p, err := body[Player](r)
		if err != nil { http.Error(w, err.Error(), 400); return }
		do(w, `UPDATE players SET nama=$2, initial_level=NULLIF($3,'') WHERE id=$1`, r.PathValue("id"), p.Nama, s(p.Level))
	})
	m.HandleFunc("DELETE /api/players/{id}", func(w http.ResponseWriter, r *http.Request) { do(w, `DELETE FROM players WHERE id=$1`, r.PathValue("id")) })

	// ---- games (sessions)
	m.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		js(w, rowsOf(`SELECT s.id,s.name,to_char(s.play_date,'YYYY-MM-DD'),s.mode,
		  (SELECT COUNT(*) FROM session_players WHERE session_id=s.id),(SELECT COUNT(*) FROM matches WHERE session_id=s.id)
		  FROM sessions s ORDER BY s.play_date DESC, s.id DESC`, func(rs *sql.Rows) any {
			var id, pl, mt int
			var name, d, mode string
			rs.Scan(&id, &name, &d, &mode, &pl, &mt)
			return map[string]any{"id": id, "name": name, "play_date": d, "mode": mode, "players": pl, "matches": mt}
		}))
	})
	m.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		b, err := body[struct{ Name string `json:"name"`; Date string `json:"play_date"`; Mode string `json:"mode"` }](r)
		if err != nil || b.Name == "" || b.Date == "" || (b.Mode != "regular" && b.Mode != "team" && b.Mode != "americano") { http.Error(w, "bad request", 400); return }
		var id int
		if err := db.QueryRow(`INSERT INTO sessions(name,play_date,mode) VALUES($1,$2::date,$3) RETURNING id`, b.Name, b.Date, b.Mode).Scan(&id); err != nil { bad(w, err); return }
		js(w, map[string]int{"id": id})
	})
	m.HandleFunc("DELETE /api/sessions/{sid}", func(w http.ResponseWriter, r *http.Request) {
		id := sidOf(r)
		for _, t := range []string{"matches", "session_players", "teams"} { db.Exec(`DELETE FROM `+t+` WHERE session_id=$1`, id) }
		do(w, `DELETE FROM sessions WHERE id=$1`, id)
	})
	m.HandleFunc("GET /api/sessions/{sid}", func(w http.ResponseWriter, r *http.Request) {
		sid := sidOf(r)
		var name, d, mode, share string
		var courts int
		if err := db.QueryRow(`SELECT name,to_char(play_date,'YYYY-MM-DD'),mode,courts,COALESCE(share_token,'') FROM sessions WHERE id=$1`, sid).Scan(&name, &d, &mode, &courts, &share); err != nil { http.Error(w, "not found", 404); return }
		pl := rowsOf(`SELECT sp.player_id,COALESCE(p.nama,sp.guest_name,sp.player_id),COALESCE(sp.level,''),sp.payment,
		  (SELECT COUNT(*) FROM matches m WHERE m.session_id=sp.session_id AND sp.player_id IN (m.p1,m.p2,m.p3,m.p4))
		  FROM session_players sp LEFT JOIN players p ON p.id=sp.player_id WHERE sp.session_id=$1 ORDER BY sp.checked_in_at, sp.player_id`, func(rs *sql.Rows) any {
			var id, nama, lv, pay string
			var g int
			rs.Scan(&id, &nama, &lv, &pay, &g)
			return map[string]any{"id": id, "nama": nama, "level": lv, "payment": pay, "games": g}
		}, sid)
		js(w, map[string]any{"id": sid, "name": name, "play_date": d, "mode": mode, "courts": courts, "share": share, "players": pl})
	})
	m.HandleFunc("PUT /api/sessions/{sid}", func(w http.ResponseWriter, r *http.Request) {
		b, err := body[struct {
			Courts int    `json:"courts"`
			Name   string `json:"name"`
		}](r)
		if err != nil { http.Error(w, "bad request", 400); return }
		if b.Name != "" { db.Exec(`UPDATE sessions SET name=$2 WHERE id=$1`, sidOf(r), b.Name) }
		if b.Courts >= 1 { db.Exec(`UPDATE sessions SET courts=$2 WHERE id=$1`, sidOf(r), b.Courts) }
		w.WriteHeader(204)
	})
	m.HandleFunc("POST /api/sessions/{sid}/players", func(w http.ResponseWriter, r *http.Request) {
		b, err := body[struct{ PlayerID string `json:"player_id"` }](r)
		if err != nil { http.Error(w, "bad request", 400); return }
		do(w, `INSERT INTO session_players(session_id,player_id,level) SELECT $1,id,initial_level FROM players WHERE id=$2 ON CONFLICT DO NOTHING`, sidOf(r), b.PlayerID)
	})
	m.HandleFunc("POST /api/sessions/{sid}/guests", func(w http.ResponseWriter, r *http.Request) {
		b, err := body[struct{ Nama string `json:"nama"`; Level string `json:"level"` }](r)
		if err != nil || b.Nama == "" { http.Error(w, "nama required", 400); return }
		do(w, `INSERT INTO session_players(session_id,player_id,level,guest_name) VALUES($1,$2,NULLIF($3,''),$4)`,
			sidOf(r), "g"+strconv.FormatInt(time.Now().UnixNano(), 36), b.Level, b.Nama)
	})
	m.HandleFunc("PUT /api/sessions/{sid}/players/{id}", func(w http.ResponseWriter, r *http.Request) {
		b, err := body[struct {
			Level   *string `json:"level"`
			Payment *string `json:"payment"`
		}](r)
		if err != nil { http.Error(w, "bad request", 400); return }
		if b.Level != nil { db.Exec(`UPDATE session_players SET level=NULLIF($3,'') WHERE session_id=$1 AND player_id=$2`, sidOf(r), r.PathValue("id"), *b.Level) }
		if b.Payment != nil { db.Exec(`UPDATE session_players SET payment=$3 WHERE session_id=$1 AND player_id=$2`, sidOf(r), r.PathValue("id"), *b.Payment) }
		w.WriteHeader(204)
	})
	m.HandleFunc("DELETE /api/sessions/{sid}/players/{id}", func(w http.ResponseWriter, r *http.Request) {
		do(w, `DELETE FROM session_players WHERE session_id=$1 AND player_id=$2`, sidOf(r), r.PathValue("id"))
	})
	m.HandleFunc("GET /api/sessions/{sid}/matches", func(w http.ResponseWriter, r *http.Request) {
		js(w, rowsOf(`SELECT p1,p2,p3,p4,score1,score2 FROM matches WHERE session_id=$1 AND finished_at IS NOT NULL ORDER BY id`, func(rs *sql.Rows) any {
			var a, b, c, d string
			var s1, s2 int
			rs.Scan(&a, &b, &c, &d, &s1, &s2)
			return map[string]any{"p": []string{a, b, c, d}, "s1": s1, "s2": s2}
		}, sidOf(r)))
	})
	m.HandleFunc("POST /api/sessions/{sid}/matches", func(w http.ResponseWriter, r *http.Request) {
		b, err := body[struct {
			Players []string `json:"players"`
			Score1  int      `json:"score1"`
			Score2  int      `json:"score2"`
			Minutes int      `json:"minutes"`
		}](r)
		if err != nil || len(b.Players) != 4 { http.Error(w, "bad request", 400); return }
		do(w, `INSERT INTO matches(session_id,court,score1,score2,p1,p2,p3,p4,finished_at,duration_min) VALUES($1,0,$2,$3,$4,$5,$6,$7,now(),NULLIF($8,0))`,
			sidOf(r), b.Score1, b.Score2, b.Players[0], b.Players[1], b.Players[2], b.Players[3], b.Minutes)
	})
	// ---- teams (team group mode)
	m.HandleFunc("GET /api/sessions/{sid}/teams", func(w http.ResponseWriter, r *http.Request) {
		js(w, rowsOf(`SELECT id,p1,p2,grp,category FROM teams WHERE session_id=$1 ORDER BY id`, func(rs *sql.Rows) any {
			var id, g int
			var p1, p2, c string
			rs.Scan(&id, &p1, &p2, &g, &c)
			return map[string]any{"id": id, "p1": p1, "p2": p2, "grp": g, "category": c}
		}, sidOf(r)))
	})
	m.HandleFunc("POST /api/sessions/{sid}/teams", func(w http.ResponseWriter, r *http.Request) {
		b, err := body[struct {
			P1       string `json:"p1"`
			P2       string `json:"p2"`
			Grp      int    `json:"grp"`
			Category string `json:"category"`
		}](r)
		if err != nil { http.Error(w, "bad request", 400); return }
		do(w, `INSERT INTO teams(session_id,p1,p2,grp,category) VALUES($1,$2,$3,$4,$5)`, sidOf(r), b.P1, b.P2, b.Grp, b.Category)
	})
	m.HandleFunc("DELETE /api/sessions/{sid}/teams/{id}", func(w http.ResponseWriter, r *http.Request) {
		do(w, `DELETE FROM teams WHERE session_id=$1 AND id=$2`, sidOf(r), r.PathValue("id"))
	})
	// ---- auth
	m.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		b, err := body[struct{ Username string `json:"username"`; Password string `json:"password"` }](r)
		var id int
		var hs string
		if err != nil || db.QueryRow(`SELECT id,password_hash FROM users WHERE username=$1`, b.Username).Scan(&id, &hs) != nil || bcrypt.CompareHashAndPassword([]byte(hs), []byte(b.Password)) != nil {
			http.Error(w, "wrong username or password", 401)
			return
		}
		tok := rnd(32)
		ua := r.UserAgent()
		if len(ua) > 200 { ua = ua[:200] }
		if _, err := db.Exec(`INSERT INTO auth_tokens(token,user_id,ua) VALUES($1,$2,$3)`, tok, id, ua); err != nil { bad(w, err); return }
		http.SetCookie(w, &http.Cookie{Name: "bt", Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 86400,
			Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"})
		js(w, map[string]string{"username": b.Username})
	})
	m.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("bt"); err == nil { db.Exec(`DELETE FROM auth_tokens WHERE token=$1`, c.Value) }
		http.SetCookie(w, &http.Cookie{Name: "bt", Path: "/", MaxAge: -1})
		w.WriteHeader(204)
	})
	m.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) { js(w, map[string]bool{"ok": true}) })
	m.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) { // add another admin (no UI yet)
		b, err := body[struct{ Username string `json:"username"`; Password string `json:"password"` }](r)
		if err != nil || b.Username == "" || len(b.Password) < 6 { http.Error(w, "username and password (6+ chars) required", 400); return }
		hs, _ := bcrypt.GenerateFromPassword([]byte(b.Password), 10)
		do(w, `INSERT INTO users(username,password_hash) VALUES($1,$2)`, b.Username, string(hs))
	})
	// ---- sharing (view only)
	m.HandleFunc("POST /api/sessions/{sid}/share", func(w http.ResponseWriter, r *http.Request) {
		var t string
		if err := db.QueryRow(`UPDATE sessions SET share_token=COALESCE(share_token,$2) WHERE id=$1 RETURNING share_token`, sidOf(r), rnd(8)).Scan(&t); err != nil { bad(w, err); return }
		js(w, map[string]string{"token": t})
	})
	m.HandleFunc("DELETE /api/sessions/{sid}/share", func(w http.ResponseWriter, r *http.Request) { do(w, `UPDATE sessions SET share_token=NULL WHERE id=$1`, sidOf(r)) })
	m.HandleFunc("PUT /api/sessions/{sid}/live", func(w http.ResponseWriter, r *http.Request) { // names only: who is playing / up next
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<16))
		if err != nil || !json.Valid(b) { http.Error(w, "bad json", 400); return }
		do(w, `UPDATE sessions SET live=$2 WHERE id=$1`, sidOf(r), string(b))
	})
	// public: matches + leaderboard data only. No player list, levels or payments.
	m.HandleFunc("GET /api/public/{token}", func(w http.ResponseWriter, r *http.Request) {
		var sid int
		var name, d, mode, live string
		if db.QueryRow(`SELECT id,name,to_char(play_date,'YYYY-MM-DD'),mode,live FROM sessions WHERE share_token=$1`, r.PathValue("token")).Scan(&sid, &name, &d, &mode, &live) != nil {
			http.Error(w, "not found", 404)
			return
		}
		names := map[string]string{}
		rowsOf(`SELECT sp.player_id,COALESCE(p.nama,sp.guest_name,sp.player_id) FROM session_players sp LEFT JOIN players p ON p.id=sp.player_id WHERE sp.session_id=$1`,
			func(rs *sql.Rows) any { var a, b string; rs.Scan(&a, &b); names[a] = b; return nil }, sid)
		nm := func(id string) string { if n, ok := names[id]; ok { return n }; return id }
		res := rowsOf(`SELECT p1,p2,p3,p4,score1,score2,COALESCE(duration_min,0) FROM matches WHERE session_id=$1 AND finished_at IS NOT NULL ORDER BY id`, func(rs *sql.Rows) any {
			var a, b, c, e string
			var s1, s2, mn int
			rs.Scan(&a, &b, &c, &e, &s1, &s2, &mn)
			return map[string]any{"t1": []string{nm(a), nm(b)}, "t2": []string{nm(c), nm(e)}, "s1": s1, "s2": s2, "min": mn}
		}, sid)
		tm := rowsOf(`SELECT p1,p2,grp,category FROM teams WHERE session_id=$1 ORDER BY id`, func(rs *sql.Rows) any {
			var a, b, c string
			var g int
			rs.Scan(&a, &b, &g, &c)
			return map[string]any{"names": []string{nm(a), nm(b)}, "grp": g, "category": c}
		}, sid)
		js(w, map[string]any{"name": name, "play_date": d, "mode": mode, "live": json.RawMessage(live), "results": res, "teams": tm})
	})
	m.HandleFunc("GET /api/sessions/{sid}/state", func(w http.ResponseWriter, r *http.Request) {
		var st string
		var v int
		if err := db.QueryRow(`SELECT state,state_v FROM sessions WHERE id=$1`, sidOf(r)).Scan(&st, &v); err != nil { http.Error(w, "not found", 404); return }
		js(w, map[string]any{"v": v, "state": json.RawMessage(st)})
	})
	m.HandleFunc("PUT /api/sessions/{sid}/state", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<18))
		if err != nil || !json.Valid(b) { http.Error(w, "bad json", 400); return }
		var v int
		if err := db.QueryRow(`UPDATE sessions SET state=$2,state_v=state_v+1 WHERE id=$1 RETURNING state_v`, sidOf(r), string(b)).Scan(&v); err != nil { bad(w, err); return }
		js(w, map[string]int{"v": v})
	})
	m.HandleFunc("GET /api/devices", func(w http.ResponseWriter, r *http.Request) { // who is logged in right now
		c, _ := r.Cookie("bt")
		js(w, rowsOf(`SELECT substr(token,1,12),ua,to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(last_seen,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),token=$1
		  FROM auth_tokens WHERE user_id=(SELECT user_id FROM auth_tokens WHERE token=$1) ORDER BY last_seen DESC`, func(rs *sql.Rows) any {
			var id, ua, cr, ls string
			var me bool
			rs.Scan(&id, &ua, &cr, &ls, &me)
			return map[string]any{"id": id, "ua": ua, "created": cr, "last_seen": ls, "current": me}
		}, c.Value))
	})
	m.HandleFunc("POST /api/devices/logout-others", func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie("bt")
		do(w, `DELETE FROM auth_tokens WHERE token<>$1 AND user_id=(SELECT user_id FROM auth_tokens WHERE token=$1)`, c.Value)
	})
	m.HandleFunc("DELETE /api/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie("bt")
		do(w, `DELETE FROM auth_tokens WHERE substr(token,1,12)=$1 AND user_id=(SELECT user_id FROM auth_tokens WHERE token=$2)`, r.PathValue("id"), c.Value)
	})
	m.Handle("/", http.FileServer(http.Dir("web")))

	h := guard(m)
	port := os.Getenv("PORT")
	if port == "" { port = "8080" }
	log.Println("listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, h))
}
