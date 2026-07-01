// DevBoard backend — a minimal Go + Gin REST API over PostgreSQL.
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var db *sql.DB

type Task struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ProjectID   int     `json:"project_id"`
	AssigneeID  *int    `json:"assignee_id"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	DueDate     *string `json:"due_date"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type Project struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     *int   `json:"owner_id"`
	CreatedAt   string `json:"created_at"`
}

func main() {
	dsn := env("POSTGRES_URL", "postgres://devboard:devboard@localhost:5432/devboard?sslmode=disable")

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("[backend] FATAL open db: %v", err)
	}
	db.SetMaxOpenConns(10)

	for i := 0; i < 30; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("[backend] waiting for postgres (%d)…", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("[backend] FATAL ping db: %v", err)
	}

	log.Println("[backend] connected to postgres")

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "backend"})
	})

	// =========================
	// ✅ FIX: API ROUTE GROUP
	// =========================
	api := r.Group("/api")
	{
		api.GET("/projects", listProjects)
		api.POST("/projects", createProject)

		api.GET("/tasks", listTasks)
		api.POST("/tasks", createTask)
		api.PATCH("/tasks/:id", updateTask)

		api.GET("/search", searchTasks)
	}

	port := env("PORT", "8080")
	log.Printf("[backend] listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("[backend] FATAL: %v", err)
	}
}