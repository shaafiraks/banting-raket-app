# Banting Raket Open Play

Go backend + Vue frontend + PostgreSQL. One Docker image serves both (see Dockerfile).

## Run locally
1. `createdb banting_raket_db`
2. Backend: `cd backend && go mod tidy && ADMIN_USER=you ADMIN_PASSWORD='your-password' DATABASE_URL="postgres://USER@localhost:5432/banting_raket_db?sslmode=disable" go run .`
   Tables, the 82 players and the first admin account are created on first start.
3. Frontend: `cd frontend && npm install && npm run dev`, open http://localhost:5173 and log in.

## Deploy (free): Neon (database) + Render (app)
Environment variables for the app: `DATABASE_URL`, `ADMIN_USER`, `ADMIN_PASSWORD`.
Set them before the first start. The admin account is only created when the users table is empty.
