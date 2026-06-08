package ui

import (
	"fmt"
	"strings"

	"tigo/pkg/git"

	"github.com/awesome-gocui/gocui"
)

var (
	gitRepo  = false
	gitDirty = false
)

type sessionChange struct {
	Action  string
	TaskID  string
	Title   string
	Details string
}

var sessionChanges []sessionChange

func trackChange(action, taskID, title, details string) {
	updateGitState()
	if action == "Open" {
		for i, c := range sessionChanges {
			if c.TaskID == taskID && c.Action == "Close" {
				// If the task was closed and is now reopened, remove the close action from sessionChanges
				sessionChanges = append(sessionChanges[:i], sessionChanges[i+1:]...)
				return
			}
		}
	}
	if action == "Close" {
		for i, c := range sessionChanges {
			if c.TaskID == taskID && c.Action == "Open" {
				// If the task was opened and is now closed, remove the open action from sessionChanges
				sessionChanges = append(sessionChanges[:i], sessionChanges[i+1:]...)
				return
			}
		}
	}
	if action == "Edit" {
		for i, c := range sessionChanges {
			if c.TaskID == taskID {
				if c.Action == "Create" {
					// If the task was created and is now edited in this session, update the title and details in sessionChanges
					sessionChanges[i].Title = title
					sessionChanges[i].Details = details
					return
				}
				if c.Action == "Edit" {
					// If the task was already edited in this session, update the title and details in sessionChanges
					sessionChanges[i].Title = title
					sessionChanges[i].Details = details
					return
				}
			}
		}
	}
	if action == "Delete" {
		foundCreate := false
		offset := 0
		for i, c := range sessionChanges {
			if c.TaskID == taskID {
				// If the task was already changed in this session, remove it from sessionChanges since it's now deleted
				sessionChanges = append(sessionChanges[:i-offset], sessionChanges[i+1-offset:]...)
				if c.Action == "Create" {
					foundCreate = true
				}
				offset++
			}
		}
		if foundCreate {
			// If the task was created and is now deleted, we can skip adding a delete action since the net effect is that the task was never created
			return
		}
	}
	sessionChanges = append(sessionChanges, sessionChange{
		Action:  action,
		TaskID:  taskID,
		Title:   title,
		Details: details,
	})
}

func clearSessionChanges() {
	sessionChanges = nil
}

func generateCommitMessage() (subject, body string) {
	n := len(sessionChanges)
	if n == 0 {
		return "tigo: ", ""
	}
	if n == 1 {
		c := sessionChanges[0]
		subject = fmt.Sprintf("tigo: %s task %s: %s", c.Action, c.TaskID, c.Title)
		body = ""
	} else {
		var (
			actionsCounter = make(map[string]int)
			bodyBuilder    strings.Builder
		)
		for _, c := range sessionChanges {
			actionsCounter[c.Action]++
			line := fmt.Sprintf("- %s task %s: %s", c.Action, c.TaskID, c.Title)
			if c.Details != "" {
				line += fmt.Sprintf(" (%s)", c.Details)
			}
			bodyBuilder.WriteString(line)
			bodyBuilder.WriteString("\n")
		}
		subject = "tigo: "
		for action, count := range actionsCounter {
			subject += fmt.Sprintf("%d %s, ", count, action)
		}
		subject = strings.TrimSuffix(subject, ", ")
		body = strings.TrimRight(bodyBuilder.String(), "\n")
	}
	return subject, body
}

func updateGitState() {
	gitRepo = git.IsGitRepo(tigoRoot)
	if gitRepo {
		dirty, err := git.IsDirty(tigoRoot)
		gitDirty = err == nil && dirty
	} else {
		gitDirty = false
	}
}

func gitStatusString() string {
	if !gitRepo {
		return ""
	}
	if gitDirty {
		return "\x1b[33m\u25cf uncommitted\x1b[0m"
	}
	return "\x1b[32m\u25cf clean\x1b[0m"
}

func promptCommit(g *gocui.Gui, v *gocui.View) error {
	if !gitRepo {
		return promptMessageBox(g, "Not a Git Repo", "\x1b[31mThe current tigo directory is not in a git repository.\x1b[0m", "", false)
	}
	updateGitState()
	if !gitDirty && len(sessionChanges) == 0 {
		return promptMessageBox(g, "Nothing to Commit", "\x1b[33mNo uncommitted changes and no session changes to commit.\x1b[0m", "", false)
	}

	maxX, maxY := g.Size()
	width := max(maxX*2/3, 40)
	x0 := maxX/2 - width/2
	titleHeight := 3
	bodyHeight := 6
	totalHeight := titleHeight + bodyHeight
	y0 := maxY/2 - totalHeight/2

	g.Cursor = true

	subject, body := generateCommitMessage()

	if v, err := g.SetView("commitSubject", x0, y0, x0+width, y0+titleHeight-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Commit Subject"
		v.Editable = true
		v.Editor = gocui.EditorFunc(oneLineEditor)
		v.Wrap = false
		fmt.Fprint(v, subject)
		v.SetCursor(len(subject), 0)

		if _, err := g.SetCurrentView("commitSubject"); err != nil {
			return err
		}
	}
	if v, err := g.SetView("commitBody", x0, y0+titleHeight, x0+width, y0+totalHeight, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Description (Press <tab> to focus)"
		v.Editable = true
		v.Wrap = true
		fmt.Fprint(v, body)
	}

	return nil
}

func submitCommit(g *gocui.Gui, v *gocui.View) error {
	titleView, err := g.View("commitSubject")
	if err != nil {
		return err
	}
	bodyView, err := g.View("commitBody")
	if err != nil {
		return err
	}

	subject := strings.TrimSpace(titleView.Buffer())
	if subject == "" {
		subject = "WIP"
	}
	description := strings.TrimSpace(bodyView.Buffer())

	fullMsg := subject
	if description != "" {
		fullMsg += "\n\n" + description
	}

	git.CommitAll(tigoRoot, fullMsg)

	clearSessionChanges()
	updateGitState()

	closeCommitDialog(g, v)
	return updateViews(g)
}

func closeCommitDialog(g *gocui.Gui, v *gocui.View) error {
	g.Cursor = false
	for _, name := range []string{"commitSubject", "commitBody"} {
		err := g.DeleteView(name)
		if err != nil {
			return err
		}
	}
	_, err := g.SetCurrentView("tasks")
	return err
}
