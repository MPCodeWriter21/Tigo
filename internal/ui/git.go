package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"tigo/pkg/git"
	"tigo/pkg/task"

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
	if !gitDirty {
		return promptMessageBox(g, "Nothing to Commit", "\x1b[33mNo uncommitted changes and no session changes to commit.\x1b[0m", "", false)
	}

	statusOut, _ := git.RunGitCommandQuiet(tigoRoot, "status", "--porcelain", ".")

	maxX, maxY := g.Size()
	width := max(maxX*2/3, 50)
	x0 := maxX/2 - width/2
	titleHeight := 3
	bodyHeight := 6
	totalHeight := titleHeight + bodyHeight
	y0 := maxY/2 - totalHeight/2

	fileListWidth := width * 3 / 10
	msgX0 := x0 + fileListWidth + 1
	msgWidth := width - fileListWidth - 1

	g.Cursor = true

	// File list view
	if v, err := g.SetView("commitFiles", x0, y0, x0+fileListWidth, y0+totalHeight, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Files to Commit"
		v.Wrap = false
		if statusOut != "" {
			for line := range strings.SplitSeq(statusOut, "\n") {
				filePath := strings.TrimSpace(line[2:])
				statusChars := line[:2]
				var color string
				switch {
				case strings.HasPrefix(statusChars, "?"):
					color = "\x1b[36m"
				case strings.HasPrefix(statusChars, "M") || strings.HasSuffix(statusChars, "M"):
					color = "\x1b[33m"
				case strings.HasPrefix(statusChars, "A"):
					color = "\x1b[32m"
				case strings.HasPrefix(statusChars, "D") || strings.HasSuffix(statusChars, "D"):
					color = "\x1b[31m"
				case strings.HasPrefix(statusChars, "R"):
					color = "\x1b[35m"
				default:
					color = "\x1b[37m"
				}
				fmt.Fprintf(v, "%s%s\x1b[0m %s\n", color, statusChars, filePath)
			}
		} else {
			fmt.Fprint(v, "\x1b[33m(empty)\x1b[0m\n")
		}
	}

	subject, body := generateCommitMessage()

	if v, err := g.SetView("commitSubject", msgX0, y0, msgX0+msgWidth, y0+titleHeight-1, 0); err != nil {
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
	if v, err := g.SetView("commitBody", msgX0, y0+titleHeight, msgX0+msgWidth, y0+totalHeight, 0); err != nil {
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
	for _, name := range []string{"commitFiles", "commitSubject", "commitBody"} {
		g.DeleteView(name)
	}
	_, err := g.SetCurrentView("tasks")
	return err
}

func showTaskBlame(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) == 0 || selectedTask >= len(tasks) {
		return nil
	}
	t := tasks[selectedTask]

	times, names, err := git.BlameTask(tigoRoot, t.ID)
	if err != nil {
		return promptMessageBox(g, "Blame Error", fmt.Sprintf("\x1b[31m%s\x1b[0m", err), "", false)
	}
	if len(names) == 0 {
		return promptMessageBox(g, "Blame Summary", "\x1b[33mNo blame data available (file may not be committed yet).\x1b[0m", "", false)
	}

	type authorStat struct {
		count    int
		lastTime time.Time
	}
	authors := make(map[string]*authorStat)
	for i, name := range names {
		s, ok := authors[name]
		if !ok {
			s = &authorStat{}
			authors[name] = s
		}
		s.count++
		if times[i].After(s.lastTime) {
			s.lastTime = times[i]
		}
	}

	var sorted []struct {
		name string
		stat *authorStat
	}
	for name, stat := range authors {
		sorted = append(sorted, struct {
			name string
			stat *authorStat
		}{name, stat})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].stat.count > sorted[j].stat.count
	})

	var sb strings.Builder
	fmt.Fprintf(&sb, "\x1b[1;36mTask\x1b[0m: %s\n", t.ID)
	fmt.Fprintf(&sb, "\x1b[1;36mLines\x1b[0m: %d\n\n", len(names))
	fmt.Fprintf(&sb, "\x1b[1;36mContributors\x1b[0m:\n")
	for _, s := range sorted {
		fmt.Fprintf(&sb, "  \x1b[33m%s\x1b[0m: %d lines (%s)\n", s.name, s.stat.count, s.stat.lastTime.Format("2006-01-02"))
	}

	return promptMessageBox(g, "Blame Summary", sb.String(), "", false)
}

func showLineBlame(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) == 0 || selectedTask >= len(tasks) {
		return nil
	}
	t := tasks[selectedTask]
	_, cy := v.Cursor()

	lineText, err := v.Line(cy)
	if err != nil {
		return nil
	}

	times, names, err := git.BlameTask(tigoRoot, t.ID)
	if err != nil {
		return promptMessageBox(g, "Blame Error", fmt.Sprintf("\x1b[31m%s\x1b[0m", err), "", false)
	}
	if len(names) == 0 {
		return nil
	}

	// Locate each metadata field in RawLines by prefix
	var titleIdx, statusIdx, priorityIdx, tagsIdx, dueIdx = -1, -1, -1, -1, -1
	for i, raw := range t.RawLines {
		switch {
		case task.TitleRegex.MatchString(raw):
			titleIdx = i
		case task.StatusRegex.MatchString(raw):
			statusIdx = i
		case task.PriorityRegex.MatchString(raw):
			priorityIdx = i
		case task.TagsRegex.MatchString(raw):
			tagsIdx = i
		case task.DueDateRegex.MatchString(raw):
			dueIdx = i
		}
	}

	// Build detail view cy -> RawLines index for the metadata section
	// cy=0: ID -> not a TASK.md line (sentinel -1)
	detailToRaw := []int{-1}
	detailToRaw = append(detailToRaw, titleIdx)    // cy=1: Title
	detailToRaw = append(detailToRaw, statusIdx)   // cy=2: Status
	detailToRaw = append(detailToRaw, priorityIdx) // cy=3: Priority
	if t.DueDate != "" {
		detailToRaw = append(detailToRaw, dueIdx) // cy=4: Due Date
	}
	detailToRaw = append(detailToRaw, tagsIdx) // cy=4 or 5: Tags
	detailToRaw = append(detailToRaw, -1)      // blank line (visual only)
	detailToRaw = append(detailToRaw, -1)      // "Description:" header (visual only)

	var taskLine int

	if cy < len(detailToRaw) {
		taskLine = detailToRaw[cy]
		if taskLine < 0 {
			return showTaskBlame(g, v)
		}
	} else {
		// Find first description line in RawLines (first non-empty, non-metadata line)
		descStart := len(t.RawLines)
		for i, raw := range t.RawLines {
			if strings.HasPrefix(raw, "# ") {
				continue
			}
			if strings.HasPrefix(raw, "- STATUS:") || strings.HasPrefix(raw, "- PRIORITY:") ||
				strings.HasPrefix(raw, "- TAGS:") || strings.HasPrefix(raw, "- DUE:") {
				continue
			}
			if strings.TrimSpace(raw) == "" {
				continue
			}
			descStart = i
			break
		}

		descLineIndex := cy - len(detailToRaw)
		taskLine = descStart + descLineIndex
		if taskLine >= len(t.RawLines) {
			return nil
		}
	}

	if taskLine < 0 || taskLine >= len(names) {
		return nil
	}

	author := names[taskLine]
	lastMod := times[taskLine].Format("2006-01-02 15:04:05")
	cleanLine := strings.TrimSpace(allANSIRegex.ReplaceAllString(lineText, ""))

	msg := fmt.Sprintf(
		"\x1b[1;36mLine %d\x1b[0m (TASK.md:%d)\n\n"+
			"\x1b[33mAuthor\x1b[0m: %s\n"+
			"\x1b[33mDate\x1b[0m:   %s\n\n"+
			"\x1b[1;37m%s\x1b[0m",
		cy, taskLine+1, author, lastMod, cleanLine)

	return promptMessageBox(g, "Line Blame", msg, "", false)
}
