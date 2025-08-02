package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// todoistAPIBase is the base URL for Todoist REST API v2
const todoistAPIBase = "https://api.todoist.com/rest/v2"

// TodoistTask represents a task from the Todoist API
type TodoistTask struct {
	// ID is the unique task identifier
	ID string `json:"id"`
	// ProjectID is the ID of the project containing this task
	ProjectID string `json:"project_id"`
	// SectionID is the ID of the section containing this task
	SectionID string `json:"section_id"`
	// Content is the task title/content
	Content string `json:"content"`
	// Description is the additional task description
	Description string `json:"description"`
	// IsCompleted indicates whether the task is completed
	IsCompleted bool `json:"is_completed"`
	// Labels is an array of label names
	Labels []string `json:"labels"`
	// Priority is the priority level (1-4, where 4 is highest)
	Priority int `json:"priority"`
	// Assignee is the user ID of the assignee
	Assignee string `json:"assignee"`
	// AssignerID is the user ID of who assigned the task
	AssignerID string `json:"assigner_id"`
	// CommentCount is the number of comments on the task
	CommentCount int `json:"comment_count"`
	// CreatedAt is when the task was created
	CreatedAt time.Time `json:"created_at"`
	// CreatorID is the user ID of task creator
	CreatorID string `json:"creator_id"`
	// Due contains due date information (optional)
	Due *Due `json:"due"`
	// Duration contains task duration (optional)
	Duration *Duration `json:"duration"`
	// URL is the URL to the task in Todoist
	URL string `json:"url"`
}

// Due represents due date information for a task
type Due struct {
	// Date is the due date in YYYY-MM-DD format
	Date string `json:"date"`
	// IsRecurring indicates whether this is a recurring task
	IsRecurring bool `json:"is_recurring"`
	// Datetime is the due datetime in UTC
	Datetime string `json:"datetime"`
	// String is the human readable due date string
	String string `json:"string"`
	// Timezone is the timezone for the due date
	Timezone string `json:"timezone"`
}

// Duration represents the estimated duration for a task
type Duration struct {
	// Amount is the duration amount
	Amount int `json:"amount"`
	// Unit is the duration unit (e.g., "minute", "hour")
	Unit string `json:"unit"`
}

// TodoistProject represents a project from the Todoist API
type TodoistProject struct {
	// ID is the unique project identifier
	ID string `json:"id"`
	// Name is the project name
	Name string `json:"name"`
	// Color is the project color
	Color string `json:"color"`
}

// TodoistClient handles communication with the Todoist API
type TodoistClient struct {
	// token is the API authentication token
	token string
	// httpClient is the HTTP client for making requests
	httpClient *http.Client
	// projects is a cache mapping project IDs to names
	projects map[string]string
}

// NewTodoistClient creates a new Todoist API client with the given token
func NewTodoistClient(token string) *TodoistClient {
	return &TodoistClient{
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second}, // 30 second timeout for API requests
		projects:   make(map[string]string),                 // Initialize empty project cache
	}
}

// GetTasks fetches all active tasks from the Todoist API
func (c *TodoistClient) GetTasks() ([]TodoistTask, error) {
	// Create HTTP GET request for tasks endpoint
	req, err := http.NewRequest("GET", todoistAPIBase+"/tasks", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers for Todoist API authentication
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	// Execute the HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if the API returned a success status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	// Parse the JSON response into TodoistTask structs
	var tasks []TodoistTask
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return tasks, nil
}

// GetProjects fetches all projects from the Todoist API
func (c *TodoistClient) GetProjects() ([]TodoistProject, error) {
	// Create HTTP GET request for projects endpoint
	req, err := http.NewRequest("GET", todoistAPIBase+"/projects", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers for Todoist API authentication
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	// Execute the HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if the API returned a success status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	// Parse the JSON response into TodoistProject structs
	var projects []TodoistProject
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return projects, nil
}

// loadProjects loads project data into the cache if not already loaded
func (c *TodoistClient) loadProjects() error {
	// Skip loading if projects are already cached
	if len(c.projects) > 0 {
		return nil
	}

	// Fetch projects from the API
	projects, err := c.GetProjects()
	if err != nil {
		return err
	}

	// Populate the cache with project ID to name mappings
	for _, project := range projects {
		c.projects[project.ID] = project.Name
	}

	return nil
}

// LoadProjectsFromCache populates the client's project cache from cached project data
func (c *TodoistClient) LoadProjectsFromCache(projects []TodoistProject) {
	// Clear existing project cache
	c.projects = make(map[string]string)
	
	// Populate cache with project ID to name mappings
	for _, project := range projects {
		c.projects[project.ID] = project.Name
	}
}

// GetProjectName returns the project name for a given project ID
// Returns "Unknown Project" if the project ID is not found in the cache
func (c *TodoistClient) GetProjectName(projectID string) string {
	if name, exists := c.projects[projectID]; exists {
		return name
	}
	return "Unknown Project"
}

// GetTodaysTasks fetches and filters tasks that are due today or overdue
// Returns tasks sorted with overdue tasks first (oldest first), then today's tasks by priority
func (c *TodoistClient) GetTodaysTasks() ([]TodoistTask, error) {
	// Load project information first to enable project name lookups
	if err := c.loadProjects(); err != nil {
		return nil, fmt.Errorf("failed to load projects: %w", err)
	}

	// Fetch all active tasks from the API
	allTasks, err := c.GetTasks()
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
			if taskDate == today || isOverdue(taskDate, today) {
				todaysTasks = append(todaysTasks, task)
			}
		}
	}

	// Sort tasks with smart ordering logic
	sort.Slice(todaysTasks, func(i, j int) bool {
		taskI := todaysTasks[i]
		taskJ := todaysTasks[j]

		// Determine if each task is overdue
		isOverdueI := isOverdue(taskI.Due.Date, today)
		isOverdueJ := isOverdue(taskJ.Due.Date, today)

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

// isOverdue checks if a task date is before today's date
// Returns false if either date cannot be parsed
func isOverdue(taskDate, today string) bool {
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

// NewTaskRequest represents the data structure for creating a new task
type NewTaskRequest struct {
	// Content is the task title/content (required)
	Content string `json:"content"`
	// Description is the additional task description (optional)
	Description string `json:"description,omitempty"`
	// ProjectID is the ID of the project to add the task to (optional)
	ProjectID string `json:"project_id,omitempty"`
	// Priority is the priority level (1-4, where 4 is highest, optional)
	Priority int `json:"priority,omitempty"`
	// Labels is an array of label names (optional)
	Labels []string `json:"labels,omitempty"`
	// DueString is a human-readable due date string (optional)
	DueString string `json:"due_string,omitempty"`
}

// CreateTask creates a new task in Todoist
func (c *TodoistClient) CreateTask(task NewTaskRequest) (*TodoistTask, error) {
	// Convert the task request to JSON
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task: %w", err)
	}

	// Create HTTP POST request for tasks endpoint
	req, err := http.NewRequest("POST", todoistAPIBase+"/tasks",
		bytes.NewBuffer(taskJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers for Todoist API authentication
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	// Execute the HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if the API returned a success status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	// Parse the JSON response into TodoistTask struct
	var createdTask TodoistTask
	if err := json.NewDecoder(resp.Body).Decode(&createdTask); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &createdTask, nil
}

// CompleteTask marks a task as completed in Todoist
func (c *TodoistClient) CompleteTask(taskID string) error {
	// Create HTTP POST request for task close endpoint
	req, err := http.NewRequest("POST", todoistAPIBase+"/tasks/"+taskID+"/close", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers for Todoist API authentication
	req.Header.Set("Authorization", "Bearer "+c.token)

	// Execute the HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if the API returned a success status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return nil
}

// DeleteTask permanently deletes a task from Todoist
func (c *TodoistClient) DeleteTask(taskID string) error {
	// Create HTTP DELETE request for task endpoint
	req, err := http.NewRequest("DELETE", todoistAPIBase+"/tasks/"+taskID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers for Todoist API authentication
	req.Header.Set("Authorization", "Bearer "+c.token)

	// Execute the HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if the API returned a success status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return nil
}

// completeTask creates a command that completes a task via Todoist API
func completeTask(client *TodoistClient, taskID string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Call the API to complete the task
		err := client.CompleteTask(taskID)
		if err != nil {
			// Return error message if API call fails
			return errorMsg(err)
		}
		// Return completed task ID on success
		return taskCompletedMsg(taskID)
	})
}

// createTaskWithDetails creates a command that creates a new task with detailed parameters
func createTaskWithDetails(client *TodoistClient, content string, priority int, projectID, deadline string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Create the task request with form data
		taskRequest := NewTaskRequest{
			Content:   content,
			Priority:  priority,
			DueString: deadline,
		}

		// Add project ID if specified
		if projectID != "" {
			taskRequest.ProjectID = projectID
		}

		// Call the API to create the task
		createdTask, err := client.CreateTask(taskRequest)
		if err != nil {
			// Return error message if API call fails
			return errorMsg(err)
		}
		// Return created task on success
		return taskCreatedMsg(*createdTask)
	})
}

// deleteTask creates a command that deletes a task via Todoist API
func deleteTask(client *TodoistClient, taskID string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Call the API to delete the task
		err := client.DeleteTask(taskID)
		if err != nil {
			// Return error message if API call fails
			return errorMsg(err)
		}
		// Return deleted task ID on success
		return taskDeletedMsg(taskID)
	})
}
