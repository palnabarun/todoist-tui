package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
)

// Global styling variables for the TUI interface
var (
	// titleStyle defines the styling for section titles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginLeft(2)

	// headerStyle defines the styling for table headers
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#374151")).
			MarginLeft(4).
			MarginBottom(1)

	// taskStyle defines the base styling for task content
	taskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#374151")).
			MarginLeft(4)

	// projectStyle defines the styling for project names
	projectStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true)

	// priorityColors maps priority levels to their display colors
	priorityColors = map[int]lipgloss.Color{
		1: lipgloss.Color("#9CA3AF"), // Low priority - light gray
		2: lipgloss.Color("#6366F1"), // Normal priority - soft indigo
		3: lipgloss.Color("#F59E0B"), // High priority - amber
		4: lipgloss.Color("#F97316"), // Urgent priority - orange (less harsh than red)
	}

	// errorStyle defines the styling for error messages
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			MarginLeft(2)

	// loadingStyle defines the styling for loading and status messages
	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			MarginLeft(2)

	// popupStyle defines the styling for the task details popup
	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Padding(1, 2).
			Foreground(lipgloss.Color("#374151"))

	// popupTitleStyle defines the styling for popup titles
	popupTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)

	// popupFieldStyle defines the styling for popup field labels
	popupFieldStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4B5563"))

	// Selection colors for highlighting selected tasks
	selectionBgColor = lipgloss.Color("#EDE9FE") // Light purple background
	selectionFgColor = lipgloss.Color("#5B21B6") // Dark purple foreground
)

// model represents the application state for the Bubble Tea TUI
type model struct {
	// tasks holds the filtered list of today's and overdue tasks
	tasks []TodoistTask
	// loading indicates whether the app is currently fetching data
	loading bool
	// error holds any error that occurred during operation
	error error
	// client is the Todoist API client instance
	client *TodoistClient
	// columns defines which table columns to display
	columns []string
	// width is the current terminal width
	width int
	// height is the current terminal height
	height int
	// selectedIndex is the index of the currently selected task (-1 if none)
	selectedIndex int
	// showingPopup indicates whether the task details popup is visible
	showingPopup bool
	// allTasks holds the complete list of tasks for navigation (overdue + today)
	allTasks []TodoistTask
	// projects holds the list of available projects
	projects []TodoistProject
	// showingCreateTask indicates whether the create task form is visible
	showingCreateTask bool
	// creating indicates whether a task is currently being created
	creating bool
	// createTaskForm holds the form state for creating new tasks
	createTaskForm createTaskFormState
	// showingDeleteConfirm indicates whether the delete confirmation dialog is visible
	showingDeleteConfirm bool
	// taskToDelete holds the ID of the task pending deletion
	taskToDelete string
}

// tasksLoadedMsg is sent when tasks have been successfully loaded from the API
type tasksLoadedMsg []TodoistTask

// projectsLoadedMsg is sent when projects have been successfully loaded from the API
type projectsLoadedMsg []TodoistProject

// errorMsg is sent when an error occurs during API operations
type errorMsg error

// taskCreatedMsg is sent when a task has been successfully created
type taskCreatedMsg TodoistTask

// taskCompletedMsg is sent when a task has been successfully completed
type taskCompletedMsg string

// taskDeletedMsg is sent when a task has been successfully deleted
type taskDeletedMsg string

// createTaskFormField represents the different fields in the create task form
type createTaskFormField int

const (
	fieldContent createTaskFormField = iota
	fieldPriority
	fieldProject
	fieldDeadline
)

// createTaskFormState holds the state of the create task form
type createTaskFormState struct {
	content            string
	priority           int    // 1-4 (1=low, 4=urgent)
	projectID          string
	projectName        string
	selectedProjectIdx int    // Index in the filtered projects list
	projectSearch      string // Search query for project filtering
	filteredProjects   []TodoistProject // Filtered list of projects based on search
	deadline           string
	activeField        createTaskFormField
}

// initialModel creates the initial application model with the specified columns
func initialModel(columns []string) model {
	// Check for required TODOIST_TOKEN environment variable
	token := os.Getenv("TODOIST_TOKEN")
	if token == "" {
		return model{
			error: fmt.Errorf("TODOIST_TOKEN environment variable is required"),
		}
	}

	// Return initialized model with default values
	return model{
		loading:              true,                    // Start in loading state
		client:               NewTodoistClient(token), // Initialize API client
		columns:              columns,                 // Store column configuration
		width:                80,                      // Default terminal width
		height:               24,                      // Default terminal height
		selectedIndex:        -1,                      // No task selected initially
		showingPopup:         false,                   // Popup hidden initially
		allTasks:             []TodoistTask{},         // Empty task list initially
		projects:             []TodoistProject{},      // Empty projects list initially
		showingCreateTask: false, // Create task form hidden initially
		creating:          false, // Not creating a task initially
		createTaskForm: createTaskFormState{
			content:            "",
			priority:           1,           // Default to low priority
			projectID:          "",          // No project selected initially
			projectName:        "Inbox",     // Default to Inbox
			selectedProjectIdx: -1,          // No project selected initially
			projectSearch:      "",          // No search query initially
			filteredProjects:   []TodoistProject{}, // Empty filtered list initially
			deadline:           "today",     // Default to today
			activeField:        fieldContent, // Start with content field active
		},
		showingDeleteConfirm: false,                   // Delete confirmation hidden initially
		taskToDelete:         "",                      // No task pending deletion initially
	}
}

// Init is called when the program starts and returns the initial command to run
func (m model) Init() tea.Cmd {
	// Don't load data if there's already an error (e.g., missing token)
	if m.error != nil {
		return nil
	}
	// Start loading both tasks and projects from the API
	return tea.Batch(loadTasks(m.client), loadProjects(m.client))
}

// loadTasks creates a command that fetches tasks from Todoist API in the background
func loadTasks(client *TodoistClient) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Call the API to get today's tasks
		tasks, err := client.GetTodaysTasks()
		if err != nil {
			// Return error message if API call fails
			return errorMsg(err)
		}
		// Return loaded tasks on success
		return tasksLoadedMsg(tasks)
	})
}

// loadProjects creates a command that fetches projects from Todoist API in the background
func loadProjects(client *TodoistClient) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Call the API to get projects
		projects, err := client.GetProjects()
		if err != nil {
			// Return error message if API call fails
			return errorMsg(err)
		}
		// Return loaded projects on success
		return projectsLoadedMsg(projects)
	})
}

// fuzzySearchProjects filters projects based on a search query using simple fuzzy matching
// Returns projects that contain all characters from the query in order (case-insensitive)
func fuzzySearchProjects(projects []TodoistProject, query string) []TodoistProject {
	if query == "" {
		return projects
	}

	var filtered []TodoistProject
	queryLower := strings.ToLower(query)
	
	for _, project := range projects {
		projectNameLower := strings.ToLower(project.Name)
		
		// Simple fuzzy matching: check if all characters from query appear in order
		queryIdx := 0
		for _, char := range projectNameLower {
			if queryIdx < len(queryLower) && char == rune(queryLower[queryIdx]) {
				queryIdx++
			}
		}
		
		// If all query characters were found in order, include this project
		if queryIdx == len(queryLower) {
			filtered = append(filtered, project)
		}
	}
	
	return filtered
}

// updateProjectFilter updates the filtered projects list and resets selection
func (m *model) updateProjectFilter() {
	m.createTaskForm.filteredProjects = fuzzySearchProjects(m.projects, m.createTaskForm.projectSearch)
	
	// Reset selection to first filtered project if available
	if len(m.createTaskForm.filteredProjects) > 0 {
		m.createTaskForm.selectedProjectIdx = 0
		project := m.createTaskForm.filteredProjects[0]
		m.createTaskForm.projectID = project.ID
		m.createTaskForm.projectName = project.Name
	} else {
		// No matches, clear selection
		m.createTaskForm.selectedProjectIdx = -1
		m.createTaskForm.projectID = ""
		m.createTaskForm.projectName = "No matches"
	}
}

// Update handles incoming messages and updates the model state accordingly
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Update terminal dimensions when window is resized
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Always handle Ctrl+C to quit
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Check for delete combination first (Cmd+Backspace on macOS, Alt+Backspace elsewhere)
		if (msg.Type == tea.KeyBackspace && msg.Alt) ||
			(msg.Type == tea.KeyBackspace && runtime.GOOS == "darwin" && msg.Alt) {
			// Handle delete for current view
			if !m.showingDeleteConfirm && !m.showingCreateTask {
				if m.selectedIndex >= 0 && m.selectedIndex < len(m.allTasks) {
					selectedTask := m.allTasks[m.selectedIndex]
					if m.showingPopup {
						m.showingPopup = false // Close popup first
					}
					m.showingDeleteConfirm = true
					m.taskToDelete = selectedTask.ID
					return m, nil
				}
			}
		}

		// Handle input based on current view state
		if m.showingDeleteConfirm {
			return m.handleDeleteConfirmInput(msg)
		} else if m.showingCreateTask {
			return m.handleCreateTaskInput(msg)
		} else if m.showingPopup {
			return m.handlePopupInput(msg)
		} else {
			return m.handleMainViewInput(msg)
		}

	case tasksLoadedMsg:
		// Handle successful task loading
		m.tasks = []TodoistTask(msg)
		m.allTasks = []TodoistTask(msg) // Store all tasks for navigation
		m.loading = false
		m.error = nil
		// Set initial selection to first task if we have tasks
		if len(m.allTasks) > 0 && m.selectedIndex == -1 {
			m.selectedIndex = 0
		}
		// Reset selection if it's out of bounds
		if m.selectedIndex >= len(m.allTasks) {
			if len(m.allTasks) > 0 {
				m.selectedIndex = 0
			} else {
				m.selectedIndex = -1
			}
		}

	case projectsLoadedMsg:
		// Handle successful project loading
		m.projects = []TodoistProject(msg)
		// Initialize filtered projects with all projects
		m.createTaskForm.filteredProjects = m.projects
		// Set default project to first one (usually Inbox) if available
		if len(m.projects) > 0 {
			m.createTaskForm.selectedProjectIdx = 0
			m.createTaskForm.projectID = m.projects[0].ID
			m.createTaskForm.projectName = m.projects[0].Name
		}

	case taskCreatedMsg:
		// Handle successful task creation
		m.creating = false
		m.showingCreateTask = false
		// Reset form state with first project if available
		defaultProjectIdx := -1
		defaultProjectID := ""
		defaultProjectName := "Inbox"
		if len(m.projects) > 0 {
			defaultProjectIdx = 0
			defaultProjectID = m.projects[0].ID
			defaultProjectName = m.projects[0].Name
		}
		m.createTaskForm = createTaskFormState{
			content:            "",
			priority:           1,
			projectID:          defaultProjectID,
			projectName:        defaultProjectName,
			selectedProjectIdx: defaultProjectIdx,
			projectSearch:      "",
			filteredProjects:   m.projects, // Reset to all projects
			deadline:           "today",
			activeField:        fieldContent,
		}
		m.loading = true

		return m, loadTasks(m.client)
	case taskCompletedMsg:
		// Handle successful task completion
		taskID := string(msg)
		// Remove the completed task from our local list
		var updatedTasks []TodoistTask
		for _, task := range m.allTasks {
			if task.ID != taskID {
				updatedTasks = append(updatedTasks, task)
			}
		}
		m.allTasks = updatedTasks
		m.tasks = updatedTasks

		// Adjust selection if needed
		if m.selectedIndex >= len(m.allTasks) {
			if len(m.allTasks) > 0 {
				m.selectedIndex = len(m.allTasks) - 1
			} else {
				m.selectedIndex = -1
			}
		}

	case taskDeletedMsg:
		// Handle successful task deletion
		taskID := string(msg)
		// Remove the deleted task from our local list
		var updatedTasks []TodoistTask
		for _, task := range m.allTasks {
			if task.ID != taskID {
				updatedTasks = append(updatedTasks, task)
			}
		}
		m.allTasks = updatedTasks
		m.tasks = updatedTasks

		// Adjust selection if needed
		if m.selectedIndex >= len(m.allTasks) {
			if len(m.allTasks) > 0 {
				m.selectedIndex = len(m.allTasks) - 1
			} else {
				m.selectedIndex = -1
			}
		}

	case errorMsg:
		// Handle error messages
		m.error = error(msg)
		m.loading = false
	}

	return m, nil
}

// View renders the current application state as a string for display
func (m model) View() string {
	var b strings.Builder

	// Display main application title
	b.WriteString(titleStyle.Render("📋 Today's Tasks & Overdue"))
	b.WriteString("\n\n")

	// Handle error state
	if m.error != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.error)))
		b.WriteString("\n\nPress Ctrl+C to quit")
		return b.String()
	}

	// Handle loading state
	if m.loading {
		b.WriteString(loadingStyle.Render("Loading tasks..."))
		b.WriteString("\n\nPress Ctrl+C to quit")
		return b.String()
	}

	// Handle empty tasks state
	if len(m.tasks) == 0 {
		b.WriteString(taskStyle.Render("🎉 No tasks due today! Great job!"))
	} else {
		// Separate tasks into overdue and today's categories
		var overdueTasks, todayTasks []TodoistTask
		for _, task := range m.tasks {
			if isTaskOverdue(task) {
				overdueTasks = append(overdueTasks, task)
			} else {
				todayTasks = append(todayTasks, task)
			}
		}

		// Keep track of task index for selection
		taskIndex := 0

		// Render overdue tasks section if any exist
		if len(overdueTasks) > 0 {
			b.WriteString(titleStyle.Render("⚠️ Overdue Tasks"))
			b.WriteString("\n")
			// Generate dynamic headers based on selected columns
			header, separator := m.generateHeaders()
			b.WriteString(headerStyle.Render(header))
			b.WriteString("\n")
			b.WriteString(headerStyle.Render(separator))
			b.WriteString("\n")

			// Render each overdue task with index
			for _, task := range overdueTasks {
				m.renderTask(task, &b, taskIndex)
				taskIndex++
			}
			b.WriteString("\n")
		}

		// Render today's tasks section if any exist
		if len(todayTasks) > 0 {
			b.WriteString(titleStyle.Render("📅 Today's Tasks"))
			b.WriteString("\n")
			// Generate dynamic headers based on selected columns
			header, separator := m.generateHeaders()
			b.WriteString(headerStyle.Render(header))
			b.WriteString("\n")
			b.WriteString(headerStyle.Render(separator))
			b.WriteString("\n")

			// Render each today's task with index
			for _, task := range todayTasks {
				m.renderTask(task, &b, taskIndex)
				taskIndex++
			}
		}
	}

	// Add footer with help text
	b.WriteString("\n")
	if len(m.allTasks) > 0 {
		deleteText := getDeleteShortcutText()
		b.WriteString(loadingStyle.Render("↑/↓ or j/k: navigate • Enter/Space: details • e: complete • " + deleteText + " • o: open • q: new task • r: refresh • ESC/Ctrl+C: quit"))
	} else {
		b.WriteString(loadingStyle.Render("Press 'r' to refresh, 'q' for new task, ESC/Ctrl+C to quit"))
	}

	// Get the main view content
	mainView := b.String()

	// If showing popup, overlay it on top of the main view
	if m.showingPopup {
		popup := m.renderTaskPopup()
		// Place popup over main view
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, mainView) + "\n" + popup
	}

	// If showing create task form, overlay it on top of the main view
	if m.showingCreateTask {
		popup := m.renderCreateTaskForm()
		// Place popup over main view
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, mainView) + "\n" + popup
	}

	// If showing delete confirmation, overlay it on top of the main view
	if m.showingDeleteConfirm {
		popup := m.renderDeleteConfirmDialog()
		// Place popup over main view
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, mainView) + "\n" + popup
	}

	return mainView
}

// getPriorityText converts numeric priority to text representation
// Priority mapping: 4=P1 (Urgent), 3=P2 (High), 2=P3 (Normal), 1=P4 (Low)
func getPriorityText(priority int) string {
	switch priority {
	case 4:
		return "P1" // Urgent
	case 3:
		return "P2" // High
	case 2:
		return "P3" // Normal
	default:
		return "P4" // Low
	}
}

// calculateColumnWidths determines optimal column widths based on terminal size
// Returns widths for priority, task, and project columns respectively
func (m model) calculateColumnWidths() (int, int, int) {
	// Calculate available width (subtract margins and padding)
	availableWidth := m.width - 8 // Account for margins and spacing

	// Define base column widths
	priorityWidth := 8 // Fixed width for priority column
	projectWidth := 20 // Preferred width for project column

	// Calculate task column width from remaining space
	taskWidth := availableWidth - priorityWidth - projectWidth - 6 // 6 for spacing between columns

	// Ensure minimum widths for readability
	if taskWidth < 20 {
		taskWidth = 20
		projectWidth = 15 // Reduce project width if terminal is narrow
	}
	if projectWidth < 10 {
		projectWidth = 10 // Minimum project width
	}

	return priorityWidth, taskWidth, projectWidth
}

// generateHeaders creates table headers and separators based on selected columns
// Returns header text and separator line for the table
func (m model) generateHeaders() (string, string) {
	// Get dynamic column widths based on terminal size
	priorityWidth, taskWidth, projectWidth := m.calculateColumnWidths()

	var headerParts []string
	var separatorParts []string

	// Generate headers and separators for each selected column
	for _, col := range m.columns {
		switch strings.ToLower(col) {
		case "priority":
			headerParts = append(headerParts, "PRIORITY")
			separatorParts = append(separatorParts, strings.Repeat("─", priorityWidth))
		case "task":
			// Pad task header to full column width
			taskHeader := "TASK" + strings.Repeat(" ", taskWidth-4)
			headerParts = append(headerParts, taskHeader)
			separatorParts = append(separatorParts, strings.Repeat("─", taskWidth))
		case "project":
			headerParts = append(headerParts, "PROJECT")
			separatorParts = append(separatorParts, strings.Repeat("─", projectWidth))
		}
	}

	// Join parts with double space separation
	header := strings.Join(headerParts, "  ")
	separator := strings.Join(separatorParts, "  ")

	return header, separator
}

// wrapText breaks text into multiple lines to fit within the specified width
// Uses word boundaries to avoid breaking words when possible
func wrapText(text string, width int) []string {
	// Return single line if text fits within width
	if len(text) <= width {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text) // Split on whitespace

	// Handle edge case of no words
	if len(words) == 0 {
		return []string{text}
	}

	// Build lines by adding words until width is reached
	currentLine := ""
	for _, word := range words {
		if len(currentLine) == 0 {
			// First word on the line
			currentLine = word
		} else if len(currentLine)+1+len(word) <= width {
			// Word fits on current line with space
			currentLine += " " + word
		} else {
			// Word doesn't fit, start new line
			lines = append(lines, currentLine)
			currentLine = word
		}
	}

	// Add the last line if it has content
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// renderTask renders a single task row in the table format
// Handles text wrapping for long task content and maintains column alignment
func (m model) renderTask(task TodoistTask, b *strings.Builder, taskIndex int) {
	// Check if this task is currently selected
	isSelected := taskIndex == m.selectedIndex

	// Get color for this task's priority level
	priorityColor := priorityColors[task.Priority]
	if priorityColor == "" {
		priorityColor = priorityColors[1] // Default to low priority color if not found
	}

	// Calculate dynamic column widths based on terminal size
	priorityWidth, taskWidth, projectWidth := m.calculateColumnWidths()

	// Prepare task content with text wrapping
	taskLines := wrapText(task.Content, taskWidth)

	// Prepare project name with truncation if needed
	projectName := m.client.GetProjectName(task.ProjectID)
	if len(projectName) > projectWidth {
		projectName = projectName[:projectWidth-3] + "..."
	}

	// Render the first line with all column data
	var firstLineColumns []string
	for _, col := range m.columns {
		switch strings.ToLower(col) {
		case "priority":
			priorityText := getPriorityText(task.Priority)
			columnStyle := taskStyle.Foreground(priorityColor).Width(priorityWidth)
			if isSelected {
				columnStyle = columnStyle.Background(selectionBgColor).Foreground(selectionFgColor)
			}
			firstLineColumns = append(firstLineColumns, columnStyle.Render(priorityText))
		case "task":
			// Use first line of wrapped text or empty string
			taskContent := ""
			if len(taskLines) > 0 {
				taskContent = taskLines[0]
			}
			columnStyle := taskStyle.Foreground(priorityColor).Width(taskWidth)
			if isSelected {
				columnStyle = columnStyle.Background(selectionBgColor).Foreground(selectionFgColor)
			}
			firstLineColumns = append(firstLineColumns, columnStyle.Render(taskContent))
		case "project":
			columnStyle := projectStyle.Width(projectWidth)
			if isSelected {
				columnStyle = columnStyle.Background(selectionBgColor).Foreground(selectionFgColor)
			}
			firstLineColumns = append(firstLineColumns, columnStyle.Render(projectName))
		}
	}

	// Join columns and write first line
	row := lipgloss.JoinHorizontal(lipgloss.Top, firstLineColumns...)
	b.WriteString(row)
	b.WriteString("\n")

	// Render additional lines for wrapped task content (if any)
	if len(taskLines) > 1 {
		for _, line := range taskLines[1:] {
			var additionalColumns []string
			// Create columns for continuation lines
			for _, col := range m.columns {
				switch strings.ToLower(col) {
				case "priority":
					// Empty space for priority column on continuation lines
					columnStyle := taskStyle.Width(priorityWidth)
					if isSelected {
						columnStyle = columnStyle.Background(selectionBgColor).Foreground(selectionFgColor)
					}
					additionalColumns = append(additionalColumns, columnStyle.Render(""))
				case "task":
					// Show wrapped text line with same styling
					columnStyle := taskStyle.Foreground(priorityColor).Width(taskWidth)
					if isSelected {
						columnStyle = columnStyle.Background(selectionBgColor).Foreground(selectionFgColor)
					}
					additionalColumns = append(additionalColumns, columnStyle.Render(line))
				case "project":
					// Empty space for project column on continuation lines
					columnStyle := projectStyle.Width(projectWidth)
					if isSelected {
						columnStyle = columnStyle.Background(selectionBgColor).Foreground(selectionFgColor)
					}
					additionalColumns = append(additionalColumns, columnStyle.Render(""))
				}
			}
			// Join and write continuation line
			additionalRow := lipgloss.JoinHorizontal(lipgloss.Top, additionalColumns...)
			b.WriteString(additionalRow)
			b.WriteString("\n")
		}
	}
}

// isTaskOverdue checks if a task is overdue by comparing its due date with today
// Returns false if the task has no due date or if date parsing fails
func isTaskOverdue(task TodoistTask) bool {
	// Skip tasks without due dates
	if task.Due == nil {
		return false
	}

	// Get today's date in YYYY-MM-DD format
	today := time.Now().Format("2006-01-02")

	// Parse task due date
	taskTime, err := time.Parse("2006-01-02", task.Due.Date)
	if err != nil {
		return false // Return false if date parsing fails
	}

	// Parse today's date
	todayTime, err := time.Parse("2006-01-02", today)
	if err != nil {
		return false // Return false if date parsing fails
	}

	// Return true if task due date is before today
	return taskTime.Before(todayTime)
}

// renderTaskPopup creates a detailed popup view for the selected task
func (m model) renderTaskPopup() string {
	// Return empty string if no task is selected or popup is not showing
	if !m.showingPopup || m.selectedIndex < 0 || m.selectedIndex >= len(m.allTasks) {
		return ""
	}

	task := m.allTasks[m.selectedIndex]
	var content strings.Builder

	// Task title
	content.WriteString(popupTitleStyle.Render("📋 Task Details"))
	content.WriteString("\n\n")

	// Task content/title
	content.WriteString(popupFieldStyle.Render("Title: "))
	content.WriteString(task.Content)
	content.WriteString("\n\n")

	// Priority
	content.WriteString(popupFieldStyle.Render("Priority: "))
	priorityText := getPriorityText(task.Priority)
	priorityDesc := map[string]string{
		"P1": "P1 (Urgent)",
		"P2": "P2 (High)",
		"P3": "P3 (Normal)",
		"P4": "P4 (Low)",
	}
	content.WriteString(priorityDesc[priorityText])
	content.WriteString("\n\n")

	// Project
	content.WriteString(popupFieldStyle.Render("Project: "))
	content.WriteString(m.client.GetProjectName(task.ProjectID))
	content.WriteString("\n\n")

	// Due date
	content.WriteString(popupFieldStyle.Render("Due Date: "))
	if task.Due != nil {
		content.WriteString(task.Due.Date)
		if task.Due.String != "" {
			content.WriteString(" (")
			content.WriteString(task.Due.String)
			content.WriteString(")")
		}
		// Add overdue indicator
		if isTaskOverdue(task) {
			content.WriteString(" ⚠️ OVERDUE")
		}
	} else {
		content.WriteString("No due date")
	}
	content.WriteString("\n\n")

	// Description (if available)
	if task.Description != "" {
		content.WriteString(popupFieldStyle.Render("Description: "))
		content.WriteString("\n")
		// Wrap description text to fit popup width
		descLines := wrapText(task.Description, 50)
		for _, line := range descLines {
			content.WriteString(line)
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	// Labels (if any)
	if len(task.Labels) > 0 {
		content.WriteString(popupFieldStyle.Render("Labels: "))
		content.WriteString(strings.Join(task.Labels, ", "))
		content.WriteString("\n\n")
	}

	// Instructions
	deleteText := getDeleteShortcutText()
	content.WriteString("Press 'e' to complete • " + deleteText + " • 'o' to open in Todoist • ESC to close")

	// Calculate popup size and position
	popupContent := content.String()
	maxWidth := 60
	if m.width < 70 {
		maxWidth = m.width - 10
	}

	// Apply popup styling with appropriate width
	styledPopup := popupStyle.Width(maxWidth).Render(popupContent)

	// Center the popup on screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, styledPopup)
}

// renderCreateTaskForm creates a form view for creating new tasks
func (m model) renderCreateTaskForm() string {
	var content strings.Builder
	form := m.createTaskForm

	// Form title
	content.WriteString(popupTitleStyle.Render("📝 Create New Task"))
	content.WriteString("\n\n")

	// Task content field
	if form.activeField == fieldContent {
		content.WriteString(popupFieldStyle.Render("→ Task: "))
	} else {
		content.WriteString(popupFieldStyle.Render("  Task: "))
	}
	if m.creating {
		content.WriteString(form.content + " (Creating...)")
	} else if form.activeField == fieldContent {
		content.WriteString(form.content + "│")
	} else {
		content.WriteString(form.content)
	}
	content.WriteString("\n\n")

	// Priority field
	if form.activeField == fieldPriority {
		content.WriteString(popupFieldStyle.Render("→ Priority: "))
	} else {
		content.WriteString(popupFieldStyle.Render("  Priority: "))
	}
	priorityText := map[int]string{
		1: "P4 (Low)",
		2: "P3 (Normal)",
		3: "P2 (High)",
		4: "P1 (Urgent)",
	}
	content.WriteString(priorityText[form.priority])
	content.WriteString("\n\n")

	// Project field
	if form.activeField == fieldProject {
		content.WriteString(popupFieldStyle.Render("→ Project: "))
	} else {
		content.WriteString(popupFieldStyle.Render("  Project: "))
	}
	
	// Show search input and selection when project field is active
	if form.activeField == fieldProject {
		// Show search query with cursor
		if form.projectSearch != "" {
			content.WriteString(fmt.Sprintf("Search: %s│", form.projectSearch))
		} else {
			content.WriteString("Search: │")
		}
		content.WriteString("\n")
		
		// Show selected project from filtered results
		if len(form.filteredProjects) > 0 {
			content.WriteString(fmt.Sprintf("Selected: ◀ %s ▶ (%d/%d)", 
				form.projectName, 
				form.selectedProjectIdx+1, 
				len(form.filteredProjects)))
		} else {
			content.WriteString("No matching projects")
		}
	} else {
		content.WriteString(form.projectName)
	}
	content.WriteString("\n\n")

	// Deadline field
	if form.activeField == fieldDeadline {
		content.WriteString(popupFieldStyle.Render("→ Deadline: "))
	} else {
		content.WriteString(popupFieldStyle.Render("  Deadline: "))
	}
	if form.activeField == fieldDeadline {
		content.WriteString(form.deadline + "│")
	} else {
		content.WriteString(form.deadline)
	}
	content.WriteString("\n\n")

	// Instructions
	if m.creating {
		content.WriteString("Creating task...")
	} else {
		content.WriteString("Tab/Arrow: navigate • Enter: create • ESC: cancel")
		content.WriteString("\n")
		switch form.activeField {
		case fieldPriority:
			content.WriteString("←/→: change priority")
		case fieldProject:
			content.WriteString("Type: search • ←/→/↑/↓: select • Backspace: clear")
		default:
			content.WriteString("Type to edit field")
		}
	}

	// Calculate popup size and position
	popupContent := content.String()
	maxWidth := 60
	if m.width < 70 {
		maxWidth = m.width - 10
	}

	// Apply popup styling with appropriate width
	styledPopup := popupStyle.Width(maxWidth).Render(popupContent)

	// Center the popup on screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, styledPopup)
}

// renderDeleteConfirmDialog creates a confirmation dialog for task deletion
func (m model) renderDeleteConfirmDialog() string {
	var content strings.Builder

	// Find the task being deleted for display
	var taskContent string
	for _, task := range m.allTasks {
		if task.ID == m.taskToDelete {
			taskContent = task.Content
			break
		}
	}

	// Dialog title
	content.WriteString(popupTitleStyle.Render("⚠️ Delete Task"))
	content.WriteString("\n\n")

	// Task content
	content.WriteString(popupFieldStyle.Render("Task: "))
	content.WriteString(taskContent)
	content.WriteString("\n\n")

	// Warning message
	content.WriteString("Are you sure you want to permanently delete this task?")
	content.WriteString("\n")
	content.WriteString("This action cannot be undone.")
	content.WriteString("\n\n")

	// Instructions
	content.WriteString("Press 'y' to confirm • 'n' or ESC to cancel")

	// Calculate popup size and position
	popupContent := content.String()
	maxWidth := 50
	if m.width < 60 {
		maxWidth = m.width - 10
	}

	// Apply popup styling with appropriate width
	styledPopup := popupStyle.Width(maxWidth).Render(popupContent)

	// Center the popup on screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, styledPopup)
}

// handleMainViewInput handles keyboard input when in the main task list view
func (m model) handleMainViewInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape":
		// ESC quits the application when in main view
		return m, tea.Quit
	case "r":
		// Refresh tasks if not currently loading and no error
		if !m.loading && m.error == nil {
			m.loading = true
			return m, loadTasks(m.client)
		}
	case "up", "k":
		// Move selection up if we have tasks
		if len(m.allTasks) > 0 {
			if m.selectedIndex <= 0 {
				m.selectedIndex = len(m.allTasks) - 1 // Wrap to bottom
			} else {
				m.selectedIndex--
			}
		}
	case "down", "j":
		// Move selection down if we have tasks
		if len(m.allTasks) > 0 {
			if m.selectedIndex >= len(m.allTasks)-1 {
				m.selectedIndex = 0 // Wrap to top
			} else {
				m.selectedIndex++
			}
		}
	case "enter", " ":
		// Show popup for selected task if we have selection
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.allTasks) {
			m.showingPopup = true
		}
	case "o", "O":
		// Open task in Todoist if we have selection
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.allTasks) {
			task := m.allTasks[m.selectedIndex]
			_ = browser.OpenURL(task.URL)
		}
	case "e", "E":
		// Complete the selected task if we have selection
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.allTasks) {
			selectedTask := m.allTasks[m.selectedIndex]
			return m, completeTask(m.client, selectedTask.ID)
		}
	case "q", "Q":
		// Show create task form
		if !m.creating {
			m.showingCreateTask = true
			// Reset form state
			m.createTaskForm = createTaskFormState{
				content:          "",
				priority:         1,
				projectID:        "",
				projectName:      "Inbox",
				selectedProjectIdx: -1,
				projectSearch:    "",
				filteredProjects: m.projects,
				deadline:         "today",
				activeField:      fieldContent,
			}
		}
		// Delete case is now handled globally above
	}
	return m, nil
}

// handlePopupInput handles keyboard input when in the task details popup
func (m model) handlePopupInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape":
		// Close popup
		m.showingPopup = false
	case "o", "O":
		// Open task in Todoist
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.allTasks) {
			task := m.allTasks[m.selectedIndex]
			_ = browser.OpenURL(task.URL)
		}
	case "e", "E":
		// Complete the selected task from popup
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.allTasks) {
			selectedTask := m.allTasks[m.selectedIndex]
			m.showingPopup = false // Close popup first
			return m, completeTask(m.client, selectedTask.ID)
		}
		// Delete case is now handled globally above
	}
	return m, nil
}

// handleCreateTaskInput handles keyboard input when in the create task form
func (m model) handleCreateTaskInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Don't handle input if currently creating task
	if m.creating {
		return m, nil
	}

	switch msg.String() {
	case "esc", "escape":
		// Cancel create task form
		m.showingCreateTask = false
		m.createTaskForm = createTaskFormState{
			content:          "",
			priority:         1,
			projectID:        "",
			projectName:      "Inbox",
			selectedProjectIdx: -1,
			projectSearch:    "",
			filteredProjects: m.projects,
			deadline:         "today",
			activeField:      fieldContent,
		}
	case "enter":
		// Submit the new task if content is not empty
		if strings.TrimSpace(m.createTaskForm.content) != "" {
			m.creating = true
			return m, createTaskWithDetails(m.client, 
				m.createTaskForm.content, 
				m.createTaskForm.priority, 
				m.createTaskForm.projectID, 
				m.createTaskForm.deadline)
		}
	case "tab", "down":
		// Move to next field
		switch m.createTaskForm.activeField {
		case fieldContent:
			m.createTaskForm.activeField = fieldPriority
		case fieldPriority:
			m.createTaskForm.activeField = fieldProject
		case fieldProject:
			m.createTaskForm.activeField = fieldDeadline
		case fieldDeadline:
			m.createTaskForm.activeField = fieldContent
		}
	case "shift+tab", "up":
		// Move to previous field
		switch m.createTaskForm.activeField {
		case fieldContent:
			m.createTaskForm.activeField = fieldDeadline
		case fieldPriority:
			m.createTaskForm.activeField = fieldContent
		case fieldProject:
			m.createTaskForm.activeField = fieldPriority
		case fieldDeadline:
			m.createTaskForm.activeField = fieldProject
		}
	case "backspace":
		// Handle backspace for current field
		switch m.createTaskForm.activeField {
		case fieldContent:
			if len(m.createTaskForm.content) > 0 {
				m.createTaskForm.content = m.createTaskForm.content[:len(m.createTaskForm.content)-1]
			}
		case fieldProject:
			if len(m.createTaskForm.projectSearch) > 0 {
				m.createTaskForm.projectSearch = m.createTaskForm.projectSearch[:len(m.createTaskForm.projectSearch)-1]
				m.updateProjectFilter()
			}
		case fieldDeadline:
			if len(m.createTaskForm.deadline) > 0 {
				m.createTaskForm.deadline = m.createTaskForm.deadline[:len(m.createTaskForm.deadline)-1]
			}
		}
	default:
		// Handle field-specific input
		switch m.createTaskForm.activeField {
		case fieldContent:
			// Add typed characters to task content
			if len(msg.String()) == 1 && msg.String() != "\x1b" {
				m.createTaskForm.content += msg.String()
			}
		case fieldPriority:
			// Handle priority changes with arrow keys
			switch msg.String() {
			case "right":
				if m.createTaskForm.priority < 4 {
					m.createTaskForm.priority++
				}
			case "left":
				if m.createTaskForm.priority > 1 {
					m.createTaskForm.priority--
				}
			}
		case fieldProject:
			// Handle project selection with arrow keys or typing for search
			switch msg.String() {
			case "right", "down":
				if len(m.createTaskForm.filteredProjects) > 0 {
					if m.createTaskForm.selectedProjectIdx < len(m.createTaskForm.filteredProjects)-1 {
						m.createTaskForm.selectedProjectIdx++
					} else {
						m.createTaskForm.selectedProjectIdx = 0 // Wrap to first project
					}
					// Update project info
					project := m.createTaskForm.filteredProjects[m.createTaskForm.selectedProjectIdx]
					m.createTaskForm.projectID = project.ID
					m.createTaskForm.projectName = project.Name
				}
			case "left", "up":
				if len(m.createTaskForm.filteredProjects) > 0 {
					if m.createTaskForm.selectedProjectIdx > 0 {
						m.createTaskForm.selectedProjectIdx--
					} else {
						m.createTaskForm.selectedProjectIdx = len(m.createTaskForm.filteredProjects) - 1 // Wrap to last project
					}
					// Update project info
					project := m.createTaskForm.filteredProjects[m.createTaskForm.selectedProjectIdx]
					m.createTaskForm.projectID = project.ID
					m.createTaskForm.projectName = project.Name
				}
			default:
				// Add typed characters to project search
				if len(msg.String()) == 1 && msg.String() != "\x1b" {
					m.createTaskForm.projectSearch += msg.String()
					m.updateProjectFilter()
				}
			}
		case fieldDeadline:
			// Add typed characters to deadline
			if len(msg.String()) == 1 && msg.String() != "\x1b" {
				m.createTaskForm.deadline += msg.String()
			}
		}
	}
	return m, nil
}

// handleDeleteConfirmInput handles keyboard input when in the delete confirmation dialog
func (m model) handleDeleteConfirmInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		// Confirm deletion
		taskID := m.taskToDelete
		m.showingDeleteConfirm = false
		m.taskToDelete = ""
		return m, deleteTask(m.client, taskID)
	case "n", "N", "esc", "escape":
		// Cancel deletion
		m.showingDeleteConfirm = false
		m.taskToDelete = ""
	}
	return m, nil
}

// main is the entry point of the application
// Handles command-line arguments and starts the TUI
func main() {
	// Define command-line flags
	var columnsFlag = flag.String("columns", "task,project", "Comma-separated list of columns to display (priority,task,project)")
	flag.Parse()

	// Parse and clean column names
	columns := strings.Split(*columnsFlag, ",")
	for i, col := range columns {
		columns[i] = strings.TrimSpace(col) // Remove whitespace
	}

	// Validate that all specified columns are supported
	validColumns := map[string]bool{"priority": true, "task": true, "project": true}
	for _, col := range columns {
		if !validColumns[strings.ToLower(col)] {
			fmt.Printf("Invalid column: %s. Valid columns are: priority, task, project\n", col)
			os.Exit(1)
		}
	}

	// Initialize and run the Bubble Tea program
	p := tea.NewProgram(initialModel(columns))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
