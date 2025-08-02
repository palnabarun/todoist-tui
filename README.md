# Todoist TUI

A Terminal User Interface (TUI) application written in Go that fetches and displays today's tasks and overdue tasks from Todoist.

This is a personal project to enhance my day-to-day Todoist workflow. Kudos to Todoist being an amazing Task tracker and for their amazing API documentation.

## Features

- 📋 Display today's tasks and overdue tasks from Todoist
- 📊 Clean table format with priority, task, and project columns
- 🗂️ Organized sections: "Overdue Tasks" and "Today's Tasks"
- 📅 Smart sorting: overdue tasks by date (oldest first), today's tasks by priority
- 🎨 Eye-friendly color scheme with text-based priority indicators (P1-P4)
- ⚙️ Configurable columns via --columns flag
- 📏 Dynamic column widths that adapt to terminal size
- 📝 Full task titles with intelligent text wrapping
- 🎯 Interactive task selection with keyboard navigation
- 📄 Detailed task popup with complete information
- 🎪 Visual highlighting of selected tasks
- ⚡ Fast and lightweight terminal interface
- 🔄 Refresh tasks with 'r' key
- ✅ Complete tasks with 'e' key
- ➕ Create new tasks with 'q' key
- 🗑️ Delete tasks with confirmation (Option+Backspace on macOS, Alt+Backspace on other platforms)
- 🎯 Clean, focused view with clear section separation

## Prerequisites

- Go 1.24.2 or later
- Todoist account and API token

## Setup

1. **Get your Todoist API Token:**
   - Go to [Todoist Integrations](https://todoist.com/prefs/integrations)
   - Find the "API token" section
   - Copy your API token

2. **Set the environment variable:**
   ```bash
   export TODOIST_TOKEN="your_api_token_here"
   ```

3. **Build the application:**
   ```bash
   go build -o todoist-tui
   ```

   Or using Mage:
   ```bash
   mage build
   ```

4. **Run the application:**
   ```bash
   ./todoist-tui
   ```

   Or in development mode:
   ```bash
   mage dev
   ```

## Command Line Options

### Column Configuration
You can customize which columns to display using the `--columns` flag:

```bash
# Show all columns (default)
./todoist-tui --columns priority,task,project

# Show only task and project
./todoist-tui --columns task,project

# Show only tasks
./todoist-tui --columns task
```

**Available columns:**
- `priority` - Priority level (P1-P4)
- `task` - Task content/title
- `project` - Project name

## Usage

### Navigation
- **↑/↓ or j/k:** Navigate up/down through tasks
- **Enter or Space:** Show detailed popup for selected task
- **ESC:** Close popup, cancel forms, or quit application (context-dependent)
- **r:** Refresh the task list
- **Ctrl+C:** Force quit from any view

### Task Management
- **e:** Complete the selected task
- **q:** Create a new task (due today)
- **o:** Open the selected task in your web browser (Todoist)
- **Option+Backspace (macOS) / Alt+Backspace (Linux/Windows):** Delete task with confirmation

### Task Selection
- The currently selected task is highlighted with a purple background
- Use arrow keys or vim-style j/k keys to move between tasks
- Tasks are numbered from top to bottom (overdue tasks first, then today's tasks)

### Create Task Form
When creating a new task (press 'q'):
- Type the task content
- **Enter:** Create the task
- **ESC:** Cancel and return to main view
- **Backspace:** Delete characters

### Delete Confirmation
When deleting a task:
- **y:** Confirm deletion (permanent)
- **n or ESC:** Cancel deletion

### Task Details Popup
The popup shows comprehensive task information:
- **Title:** Full task content
- **Priority:** P1-P4 with description
- **Project:** Associated project name
- **Due Date:** Date with overdue indicator if applicable
- **Description:** Full task description (if provided)
- **Labels:** Associated labels (if any)
- **Created:** Task creation date and time
- **URL:** Direct link to task in Todoist

Available actions in popup:
- **e:** Complete task
- **Option+Backspace (macOS) / Alt+Backspace (other):** Delete task
- **o:** Open in browser
- **ESC:** Close popup

## Priority Indicators

- **P1** - Urgent (Orange)
- **P2** - High (Amber)
- **P3** - Normal (Indigo)
- **P4** - Low (Gray)

## How it Works

The application:
1. Fetches all active tasks and projects from Todoist API
2. Filters tasks that are due today or overdue
3. Sorts tasks with overdue tasks by date (oldest first), and today's tasks by priority
4. Organizes tasks into two clear sections: "Overdue Tasks" and "Today's Tasks"
5. Displays tasks in a responsive table format with Priority (P1-P4), Task content, and Project columns
6. Provides interactive task selection with keyboard navigation and visual highlighting
7. Shows detailed task information in an overlay popup with complete task data
8. Adapts column widths to terminal size - shows full task titles when space allows, wraps when needed
9. Uses readable color-coding for priority levels while maintaining good contrast
10. Allows real-time refresh without restarting the application

## Environment Variables

- `TODOIST_TOKEN` - Your Todoist API token (required)

## Error Handling

The application handles common errors gracefully:
- Missing API token
- Network connectivity issues
- Invalid API responses
- Rate limiting (30-second timeout)

## Development

### Building with Mage

This project uses [Mage](https://magefile.org/) for build automation. Available targets:

```bash
# Show all available targets
mage list

# Build for current platform
mage build

# Build for all platforms
mage buildall

# Run quality checks
mage check

# Run golangci-lint in container
mage lintci

# Run in development mode
mage dev

# Prepare a release
mage release
```

### Running Tests

```bash
# Run tests
mage test

# Run all quality checks (lint + test)
mage check
```

## Dependencies

### External Services
- [Todoist](https://todoist.com/) - Task management service
- [Todoist REST API v2](https://developer.todoist.com/rest/v2/) - API for task and project data

### Go Libraries
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Styling and layout
- [Mage](https://github.com/magefile/mage) - Build automation

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
