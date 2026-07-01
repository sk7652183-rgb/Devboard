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
	r.Run(":" + port)
}

// ✅ MUST be OUTSIDE main()
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}