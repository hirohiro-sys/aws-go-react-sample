package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type Todo struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type server struct {
	db *sql.DB
}

func main() {
	db, err := openDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := waitAndMigrate(db); err != nil {
		log.Fatal(err)
	}

	s := &server{db: db}
	router := mux.NewRouter()
	router.HandleFunc("/health", s.health).Methods(http.MethodGet)
	router.HandleFunc("/api/todos", s.listTodos).Methods(http.MethodGet)
	router.HandleFunc("/api/todos", s.createTodo).Methods(http.MethodPost)
	router.HandleFunc("/api/todos/{id}", s.updateTodo).Methods(http.MethodPatch)
	router.HandleFunc("/api/todos/{id}", s.deleteTodo).Methods(http.MethodDelete)

	staticDir := getenv("STATIC_DIR", "/static")
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		router.PathPrefix("/").Handler(spaHandler(staticDir))
	}

	addr := ":" + getenv("PORT", "8000")
	log.Print("listening on ", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func openDB() (*sql.DB, error) {
	password, err := os.ReadFile(getenv("DB_PASSWORD_FILE", "/run/secrets/db-password"))
	if err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		url.QueryEscape(getenv("DB_USER", "postgres")),
		url.QueryEscape(strings.TrimSpace(string(password))),
		getenv("DB_HOST", "db"),
		getenv("DB_PORT", "5432"),
		url.PathEscape(getenv("DB_NAME", "example")),
	)
	return sql.Open("postgres", dsn)
}

func waitAndMigrate(db *sql.DB) error {
	var err error
	for i := 0; i < 30; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id SERIAL PRIMARY KEY,
			title VARCHAR NOT NULL,
			completed BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(); err != nil {
		writeError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) listTodos(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT id, title, completed, created_at, updated_at
		FROM todos
		ORDER BY created_at DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list todos")
		return
	}
	defer rows.Close()

	todos := []Todo{}
	for rows.Next() {
		var todo Todo
		if err := rows.Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.CreatedAt, &todo.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list todos")
			return
		}
		todos = append(todos, todo)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list todos")
		return
	}
	writeJSON(w, http.StatusOK, todos)
}

func (s *server) createTodo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	var todo Todo
	err := s.db.QueryRow(`
		INSERT INTO todos (title) VALUES ($1)
		RETURNING id, title, completed, created_at, updated_at
	`, title).Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.CreatedAt, &todo.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create todo")
		return
	}
	writeJSON(w, http.StatusCreated, todo)
}

func (s *server) updateTodo(w http.ResponseWriter, r *http.Request) {
	id, ok := todoID(w, r)
	if !ok {
		return
	}
	var body struct {
		Title     *string `json:"title"`
		Completed *bool   `json:"completed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	var title any
	var completed any
	if body.Title != nil {
		trimmed := strings.TrimSpace(*body.Title)
		if trimmed == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}
		title = trimmed
	}
	if body.Completed != nil {
		completed = *body.Completed
	}
	if title == nil && completed == nil {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	var todo Todo
	err := s.db.QueryRow(`
		UPDATE todos
		SET title = COALESCE($2, title),
		    completed = COALESCE($3, completed),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, title, completed, created_at, updated_at
	`, id, title, completed).Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.CreatedAt, &todo.UpdatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "todo not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update todo")
		return
	}
	writeJSON(w, http.StatusOK, todo)
}

func (s *server) deleteTodo(w http.ResponseWriter, r *http.Request) {
	id, ok := todoID(w, r)
	if !ok {
		return
	}
	result, err := s.db.Exec(`DELETE FROM todos WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete todo")
		return
	}
	count, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete todo")
		return
	}
	if count == 0 {
		writeError(w, http.StatusNotFound, "todo not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func todoID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
