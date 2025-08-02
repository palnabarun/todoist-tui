package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	_ "github.com/mattn/go-sqlite3"
)

// CacheDB handles SQLite caching for tasks and projects
type CacheDB struct {
	db *sql.DB
}

// cacheRefreshedMsg is sent when cache has been refreshed with new data
type cacheRefreshedMsg struct {
	tasks    []TodoistTask
	projects []TodoistProject
}

// NewCacheDB creates and initializes a new cache database
func NewCacheDB() (*CacheDB, error) {
	// Create cache directory in user's cache dir
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		// Fallback to home directory
		homeDir, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return nil, fmt.Errorf("failed to get cache directory: %w", err)
		}
		cacheDir = filepath.Join(homeDir, ".cache")
	}

	appCacheDir := filepath.Join(cacheDir, "todoist-tui")
	if err := os.MkdirAll(appCacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	dbPath := filepath.Join(appCacheDir, "cache.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	cache := &CacheDB{db: db}
	if err := cache.createTables(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return cache, nil
}

// Close closes the database connection
func (c *CacheDB) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// createTables creates the necessary database tables
func (c *CacheDB) createTables() error {
	// Tasks table
	tasksSQL := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		content TEXT NOT NULL,
		project_id TEXT,
		priority INTEGER,
		due_date TEXT,
		due_string TEXT,
		is_completed BOOLEAN,
		labels TEXT, -- JSON array
		description TEXT,
		url TEXT,
		created_at TEXT,
		cached_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Projects table
	projectsSQL := `
	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		color TEXT,
		cached_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Cache metadata table
	metadataSQL := `
	CREATE TABLE IF NOT EXISTS cache_metadata (
		key TEXT PRIMARY KEY,
		value TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	for _, sql := range []string{tasksSQL, projectsSQL, metadataSQL} {
		if _, err := c.db.Exec(sql); err != nil {
			return err
		}
	}

	return nil
}

// SaveTasks saves tasks to the cache
func (c *CacheDB) SaveTasks(tasks []TodoistTask) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Clear existing tasks
	if _, err := tx.Exec("DELETE FROM tasks"); err != nil {
		return err
	}

	// Insert new tasks
	stmt, err := tx.Prepare(`
		INSERT INTO tasks (id, content, project_id, priority, due_date, due_string,
			is_completed, labels, description, url, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, task := range tasks {
		var dueDate, dueString string
		if task.Due != nil {
			dueDate = task.Due.Date
			dueString = task.Due.String
		}

		labelsJSON, _ := json.Marshal(task.Labels)

		_, err := stmt.Exec(
			task.ID,
			task.Content,
			task.ProjectID,
			task.Priority,
			dueDate,
			dueString,
			task.IsCompleted,
			string(labelsJSON),
			task.Description,
			task.URL,
			task.CreatedAt.Format(time.RFC3339),
		)
		if err != nil {
			return err
		}
	}

	// Update cache timestamp
	if _, err := tx.Exec(
		"INSERT OR REPLACE INTO cache_metadata (key, value) VALUES ('tasks_last_updated', ?)",
		time.Now().Format(time.RFC3339),
	); err != nil {
		return err
	}

	return tx.Commit()
}

// SaveProjects saves projects to the cache
func (c *CacheDB) SaveProjects(projects []TodoistProject) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Clear existing projects
	if _, err := tx.Exec("DELETE FROM projects"); err != nil {
		return err
	}

	// Insert new projects
	stmt, err := tx.Prepare("INSERT INTO projects (id, name, color) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, project := range projects {
		_, err := stmt.Exec(project.ID, project.Name, project.Color)
		if err != nil {
			return err
		}
	}

	// Update cache timestamp
	if _, err := tx.Exec(
		"INSERT OR REPLACE INTO cache_metadata (key, value) VALUES ('projects_last_updated', ?)",
		time.Now().Format(time.RFC3339),
	); err != nil {
		return err
	}

	return tx.Commit()
}

// LoadTasks loads tasks from the cache
func (c *CacheDB) LoadTasks() ([]TodoistTask, error) {
	rows, err := c.db.Query(`
		SELECT id, content, project_id, priority, due_date, due_string,
			is_completed, labels, description, url, created_at
		FROM tasks
		ORDER BY priority DESC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tasks []TodoistTask
	for rows.Next() {
		var task TodoistTask
		var dueDate, dueString, labelsJSON, createdAtStr string

		err := rows.Scan(
			&task.ID,
			&task.Content,
			&task.ProjectID,
			&task.Priority,
			&dueDate,
			&dueString,
			&task.IsCompleted,
			&labelsJSON,
			&task.Description,
			&task.URL,
			&createdAtStr,
		)
		if err != nil {
			return nil, err
		}

		// Parse due date if present
		if dueDate != "" {
			task.Due = &Due{
				Date:   dueDate,
				String: dueString,
			}
		}

		// Parse labels
		if labelsJSON != "" {
			_ = json.Unmarshal([]byte(labelsJSON), &task.Labels)
		}

		// Parse created at
		if createdAt, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			task.CreatedAt = createdAt
		}

		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

// LoadTodaysTasks loads tasks from cache and applies the same filtering and sorting as the API
// Returns tasks sorted with overdue tasks first (oldest first), then today's tasks by priority
func (c *CacheDB) LoadTodaysTasks() ([]TodoistTask, error) {
	// Load all tasks from cache
	allTasks, err := c.LoadTasks()
	if err != nil {
		return nil, err
	}
	
	// Get today's date in YYYY-MM-DD format for comparison
	today := time.Now().Format("2006-01-02")
	var todaysTasks []TodoistTask
	
	// Filter tasks to include only those due today or overdue
	for _, task := range allTasks {
		if task.Due != nil {
			taskDate := task.Due.Date
			// Include task if it's due today or overdue
			if taskDate == today || isCacheTaskOverdue(taskDate, today) {
				todaysTasks = append(todaysTasks, task)
			}
		}
	}
	
	// Sort tasks with smart ordering logic (same as API)
	sort.Slice(todaysTasks, func(i, j int) bool {
		taskI := todaysTasks[i]
		taskJ := todaysTasks[j]
		
		// Determine if each task is overdue
		isOverdueI := isCacheTaskOverdue(taskI.Due.Date, today)
		isOverdueJ := isCacheTaskOverdue(taskJ.Due.Date, today)
		
		// Prioritize overdue tasks over today's tasks
		if isOverdueI && !isOverdueJ {
			return true
		}
		if !isOverdueI && isOverdueJ {
			return false
		}
		
		// Parse dates for comparison
		dateI, errI := time.Parse("2006-01-02", taskI.Due.Date)
		dateJ, errJ := time.Parse("2006-01-02", taskJ.Due.Date)
		
		// Fallback to priority sorting if date parsing fails
		if errI != nil || errJ != nil {
			return taskI.Priority > taskJ.Priority
		}
		
		// For overdue tasks: sort by date (oldest first)
		if isOverdueI && isOverdueJ {
			return dateI.Before(dateJ)
		}
		
		// For today's tasks: sort by priority (higher priority first)
		return taskI.Priority > taskJ.Priority
	})
	
	return todaysTasks, nil
}

// isCacheTaskOverdue checks if a task date is before today's date (same logic as API)
// Returns false if either date cannot be parsed
func isCacheTaskOverdue(taskDate, today string) bool {
	// Parse the task due date
	taskTime, err := time.Parse("2006-01-02", taskDate)
	if err != nil {
		return false
	}
	
	// Parse today's date
	todayTime, err := time.Parse("2006-01-02", today)
	if err != nil {
		return false
	}
	
	// Return true if task date is before today
	return taskTime.Before(todayTime)
}

// LoadProjects loads projects from the cache
func (c *CacheDB) LoadProjects() ([]TodoistProject, error) {
	rows, err := c.db.Query("SELECT id, name, color FROM projects ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var projects []TodoistProject
	for rows.Next() {
		var project TodoistProject
		err := rows.Scan(&project.ID, &project.Name, &project.Color)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, rows.Err()
}

// IsStale checks if the cache is older than the specified duration
func (c *CacheDB) IsStale(cacheType string, maxAge time.Duration) bool {
	var lastUpdated string
	key := cacheType + "_last_updated"

	err := c.db.QueryRow(
		"SELECT value FROM cache_metadata WHERE key = ?",
		key,
	).Scan(&lastUpdated)

	if err != nil {
		return true // No cache data, consider stale
	}

	updatedTime, err := time.Parse(time.RFC3339, lastUpdated)
	if err != nil {
		return true // Invalid timestamp, consider stale
	}

	return time.Since(updatedTime) > maxAge
}

// refreshCacheInBackground refreshes both tasks and projects cache in the background
func refreshCacheInBackground(client *TodoistClient, cache *CacheDB) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Fetch fresh data from API
		tasks, err := client.GetTodaysTasks()
		if err != nil {
			return errorMsg(fmt.Errorf("failed to refresh tasks cache: %w", err))
		}

		projects, err := client.GetProjects()
		if err != nil {
			return errorMsg(fmt.Errorf("failed to refresh projects cache: %w", err))
		}

		// Save to cache
		if err := cache.SaveTasks(tasks); err != nil {
			return errorMsg(fmt.Errorf("failed to save tasks to cache: %w", err))
		}

		if err := cache.SaveProjects(projects); err != nil {
			return errorMsg(fmt.Errorf("failed to save projects to cache: %w", err))
		}

		return cacheRefreshedMsg{
			tasks:    tasks,
			projects: projects,
		}
	})
}
