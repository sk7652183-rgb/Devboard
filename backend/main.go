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

// ===================== MODELS =====================

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

// ===================== MAIN =====================

func main() {
	dsn := env("POSTGRES_URL", "postgres://devboard:devboard@localhost:5432/devboard?sslmode=disable")

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("[backend] DB error: %v", err)
	}

	db.SetMaxOpenConns(10)

	for i := 0; i < 30; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API GROUP
	api := r.Group("/api")
	{
		api.GET("/projects", listProjects)
		api.POST("/projects", createProject)

		api.GET("/tasks", listTasks)
		api.POST("/tasks", createTask)
		api.PATCH("/tasks/:id", updateTask)

		api.GET("/search", searchTasks)
	}

	r.Run(":8080")
}

// ===================== HANDLERS =====================

func listProjects(c *gin.Context) {
	rows, err := db.Query(`SELECT id, name, COALESCE(description,''), owner_id, created_at FROM projects ORDER BY id`)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var created time.Time

		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &created); err != nil {
			fail(c, err)
			return
		}
		p.CreatedAt = created.Format(time.RFC3339)
		projects = append(projects, p)
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func createProject(c *gin.Context) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		OwnerID     *int   `json:"owner_id"`
	}

	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}

	var p Project
	var created time.Time

	err := db.QueryRow(`
		INSERT INTO projects (name, description, owner_id)
		VALUES ($1,$2,$3)
		RETURNING id,name,COALESCE(description,''),owner_id,created_at`,
		body.Name, body.Description, body.OwnerID,
	).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &created)

	if err != nil {
		fail(c, err)
		return
	}

	p.CreatedAt = created.Format(time.RFC3339)
	c.JSON(http.StatusCreated, p)
}

func listTasks(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id required"})
		return
	}

	rows, err := db.Query(taskSelect+" WHERE project_id=$1 ORDER BY id", projectID)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	tasks, err := scanTasks(rows)
	if err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func createTask(c *gin.Context) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		ProjectID   int    `json:"project_id"`
		Status      string `json:"status"`
		Priority    string `json:"priority"`
	}

	if err := c.ShouldBindJSON(&body); err != nil || body.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	if body.Status == "" {
		body.Status = "todo"
	}
	if body.Priority == "" {
		body.Priority = "medium"
	}

	row := db.QueryRow(
		`INSERT INTO tasks (title,description,project_id,status,priority)
		 VALUES ($1,$2,$3,$4,$5)`+taskReturning,
		body.Title, body.Description, body.ProjectID, body.Status, body.Priority,
	)

	task, err := scanTask(row)
	if err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusCreated, task)
}

func updateTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var patch map[string]interface{}
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}

	allowed := map[string]bool{"title": true, "description": true, "status": true, "priority": true}

	sets := []string{}
	args := []interface{}{}
	i := 1

	for k, v := range patch {
		if !allowed[k] {
			continue
		}
		sets = append(sets, k+"=$"+strconv.Itoa(i))
		args = append(args, v)
		i++
	}

	args = append(args, id)

	query := "UPDATE tasks SET " + join(sets, ", ") +
		" WHERE id=$" + strconv.Itoa(i) + taskReturning

	row := db.QueryRow(query, args...)

	task, err := scanTask(row)
	if err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, task)
}

func searchTasks(c *gin.Context) {
	q := c.Query("q")
	projectID, _ := strconv.Atoi(c.Query("project_id"))

	rows, err := db.Query(
		taskSelect+" WHERE project_id=$1 AND title ILIKE '%'||$2||'%'",
		projectID, q,
	)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	tasks, _ := scanTasks(rows)
	c.JSON(http.StatusOK, gin.H{"results": tasks})
}

// ===================== HELPERS =====================

const taskSelect = `SELECT id,title,COALESCE(description,''),project_id,assignee_id,
status,priority,due_date,created_at,updated_at FROM tasks`

const taskReturning = ` RETURNING id,title,COALESCE(description,''),project_id,assignee_id,
status,priority,due_date,created_at,updated_at`

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fail(c *gin.Context, err error) {
	log.Println(err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

// (scanTask, scanTasks, join functions assumed same as your original)