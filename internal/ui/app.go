package ui

import (
	"context"
	"fmt"
	"lintree/internal/format"
	"lintree/internal/scanner"
	"lintree/internal/treemap"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

const (
	sidebarWidth    = 30
	breadcrumbH     = 1
	statusBarH      = 1
	maxVisibleCells = 200
	redrawInterval  = 100 * time.Millisecond // 10fps, plenty for a TUI
)

// App is the main TUI application.
type App struct {
	screen   tcell.Screen
	root     *scanner.FileNode
	focus    *scanner.FileNode
	navStack []*scanner.FileNode
	cursor   int
	cells    []treemap.Cell
	scanning bool
	progress scanner.Progress
	width    int
	height   int
	cancel   context.CancelFunc
	showHelp bool
	dirty    bool // marks that a redraw is needed

	// Progress bar animation state
	progressPhase int // animation ticker for indeterminate bar
}

// Run starts the TUI application.
func Run(rootPath string, fast bool) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("creating screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("initializing screen: %w", err)
	}
	screen.EnableMouse()
	defer screen.Fini()

	w, h := screen.Size()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := &App{
		screen:   screen,
		scanning: true,
		width:    w,
		height:   h,
		cancel:   cancel,
		dirty:    true,
	}

	// Start scan
	progCh, resultCh, errCh := scanner.Scan(ctx, rootPath, fast)

	// Pipe tcell events into a channel so we can select on them
	eventCh := make(chan tcell.Event, 100)
	go func() {
		for {
			ev := screen.PollEvent()
			if ev == nil {
				return
			}
			eventCh <- ev
		}
	}()

	// Redraw ticker: limits redraws to a fixed rate
	redrawTicker := time.NewTicker(redrawInterval)
	defer redrawTicker.Stop()

	for {
		select {
		case ev := <-eventCh:
			// Drain all queued events before redrawing
			if app.processEvent(ev) {
				return nil
			}
		drainLoop:
			for {
				select {
				case ev2 := <-eventCh:
					if app.processEvent(ev2) {
						return nil
					}
				default:
					break drainLoop
				}
			}

		case p, ok := <-progCh:
			if !ok {
				progCh = nil // stop selecting on closed channel
			} else {
				app.progress = p
				app.dirty = true
			}

		case result, ok := <-resultCh:
			if !ok {
				resultCh = nil
			} else if result != nil {
				app.root = result
				app.focus = result
				app.scanning = false
				app.rebuildLayout()
				app.dirty = true
			}

		case scanErr, ok := <-errCh:
			if !ok {
				errCh = nil
			} else if scanErr != nil {
				screen.Fini()
				return fmt.Errorf("scan error: %w", scanErr)
			}

		case <-redrawTicker.C:
			// Advance animation phase during scanning
			if app.scanning {
				app.progressPhase++
				app.dirty = true
			}
			if app.dirty {
				app.draw()
				app.dirty = false
			}
		}
	}
}

// processEvent handles a single event. Returns true if the app should quit.
func (a *App) processEvent(ev tcell.Event) bool {
	switch e := ev.(type) {
	case *tcell.EventResize:
		a.width, a.height = e.Size()
		a.screen.Sync()
		a.rebuildLayout()
		a.dirty = true
	case *tcell.EventKey:
		if a.handleKey(e) {
			return true
		}
		a.dirty = true
	case *tcell.EventMouse:
		a.handleMouse(e)
		a.dirty = true
	}
	return false
}

func (a *App) handleKey(ev *tcell.EventKey) bool {
	if a.showHelp {
		a.showHelp = false
		return false
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		if a.focus != a.root && a.focus != nil {
			a.goBack()
		} else {
			return true
		}
	case tcell.KeyCtrlC:
		return true
	case tcell.KeyEnter:
		a.drillIn()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		a.goBack()
	case tcell.KeyUp:
		a.moveCursor(-1, false)
	case tcell.KeyDown:
		a.moveCursor(1, false)
	case tcell.KeyLeft:
		a.moveCursor(-1, true)
	case tcell.KeyRight:
		a.moveCursor(1, true)
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'q':
			return true
		case 'j':
			a.moveCursor(1, false)
		case 'k':
			a.moveCursor(-1, false)
		case 'h':
			a.goBack()
		case 'l':
			a.drillIn()
		case '?':
			a.showHelp = !a.showHelp
		}
	}
	return false
}

func (a *App) handleMouse(ev *tcell.EventMouse) {
	if ev.Buttons()&tcell.Button1 == 0 {
		return
	}
	mx, my := ev.Position()
	for i, cell := range a.cells {
		r := cell.Rect
		if mx >= r.X && mx < r.X+r.W && my >= r.Y+breadcrumbH && my < r.Y+r.H+breadcrumbH {
			a.cursor = i
			break
		}
	}
}

func (a *App) moveCursor(delta int, horizontal bool) {
	if len(a.cells) == 0 {
		return
	}
	if horizontal {
		// Find spatially adjacent cell
		cur := a.cells[a.cursor].Rect
		curMidX := cur.X + cur.W/2
		curMidY := cur.Y + cur.H/2
		best := -1
		bestDist := 1<<31 - 1
		for i, c := range a.cells {
			if i == a.cursor {
				continue
			}
			midX := c.Rect.X + c.Rect.W/2
			midY := c.Rect.Y + c.Rect.H/2
			if (delta > 0 && midX <= curMidX) || (delta < 0 && midX >= curMidX) {
				continue
			}
			dist := abs(midX-curMidX)*2 + abs(midY-curMidY)
			if dist < bestDist {
				bestDist = dist
				best = i
			}
		}
		if best >= 0 {
			a.cursor = best
		}
	} else {
		a.cursor += delta
		if a.cursor < 0 {
			a.cursor = 0
		}
		if a.cursor >= len(a.cells) {
			a.cursor = len(a.cells) - 1
		}
	}
}

func (a *App) drillIn() {
	if len(a.cells) == 0 || a.cursor >= len(a.cells) {
		return
	}
	node := a.cells[a.cursor].Node
	if node != nil && node.IsDir && len(node.Children) > 0 {
		a.navStack = append(a.navStack, a.focus)
		a.focus = node
		a.cursor = 0
		a.rebuildLayout()
	}
}

func (a *App) goBack() {
	if len(a.navStack) > 0 {
		a.focus = a.navStack[len(a.navStack)-1]
		a.navStack = a.navStack[:len(a.navStack)-1]
		a.cursor = 0
		a.rebuildLayout()
	}
}

func (a *App) rebuildLayout() {
	if a.focus == nil {
		return
	}
	tmW := a.width - sidebarWidth
	if tmW < 10 {
		tmW = a.width
	}
	tmH := a.height - breadcrumbH - statusBarH
	if tmH < 3 {
		tmH = 3
	}

	children := a.focus.TopChildren(maxVisibleCells)
	a.cells = treemap.Layout(children, treemap.Rect{X: 0, Y: 0, W: tmW, H: tmH}, 0)
	if a.cursor >= len(a.cells) {
		a.cursor = max(0, len(a.cells)-1)
	}
}

func (a *App) draw() {
	a.screen.Clear()

	if a.scanning {
		a.drawScanProgress()
	} else if a.focus != nil {
		a.drawBreadcrumb()
		a.drawTreemap()
		a.drawSidebar()
		a.drawStatusBar()
		if a.showHelp {
			a.drawHelp()
		}
	}

	a.screen.Show()
}

func (a *App) drawScanProgress() {
	dimStyle := tcell.StyleDefault.Foreground(tcell.NewRGBColor(92, 99, 112))
	accentStyle := tcell.StyleDefault.Foreground(tcell.NewRGBColor(86, 182, 194))
	boldAccent := accentStyle.Bold(true)

	// Spinner
	spinChars := []rune{'\u280B', '\u2819', '\u2839', '\u2838', '\u283C', '\u2834', '\u2826', '\u2827', '\u2807', '\u280F'}
	spinIdx := a.progressPhase % len(spinChars)
	spinner := string(spinChars[spinIdx])

	y := a.height/2 - 3
	if y < 1 {
		y = 1
	}

	// Line 1: spinner + title
	titleStr := spinner + "  Scanning filesystem..."
	titleX := (a.width - utf8.RuneCountInString(titleStr)) / 2
	if titleX < 2 {
		titleX = 2
	}
	drawStr(a.screen, titleX, y, boldAccent, titleStr)
	y += 2

	// Line 2: indeterminate progress bar
	barWidth := a.width - 8
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 60 {
		barWidth = 60
	}
	bar := a.buildProgressBar(barWidth)
	barX := (a.width - utf8.RuneCountInString(bar)) / 2
	if barX < 2 {
		barX = 2
	}
	drawStr(a.screen, barX, y, accentStyle, bar)
	y += 2

	// Line 3: stats
	statsLine := fmt.Sprintf("Dirs: %s  Files: %s  Size: %s",
		format.Count(int64(a.progress.DirsScanned)), format.Count(int64(a.progress.FilesFound)),
		format.Size(a.progress.BytesFound))
	statsX := (a.width - utf8.RuneCountInString(statsLine)) / 2
	if statsX < 2 {
		statsX = 2
	}
	drawStr(a.screen, statsX, y, accentStyle, statsLine)
	y += 2

	// Line 4: current path (centered, truncated with ellipsis)
	path := a.progress.CurrentPath
	maxW := a.width - 6
	if maxW < 10 {
		maxW = 10
	}
	pathRunes := []rune(path)
	if len(pathRunes) > maxW {
		path = "..." + string(pathRunes[len(pathRunes)-maxW+3:])
	}
	pathX := (a.width - utf8.RuneCountInString(path)) / 2
	if pathX < 2 {
		pathX = 2
	}
	drawStr(a.screen, pathX, y, dimStyle, path)
}

// buildProgressBar creates an animated indeterminate progress bar string.
func (a *App) buildProgressBar(width int) string {
	highlightWidth := width / 4
	if highlightWidth < 3 {
		highlightWidth = 3
	}

	// Compute position: bounce between 0 and (width - highlightWidth)
	travel := width - highlightWidth
	if travel < 1 {
		travel = 1
	}
	cycle := travel * 2
	pos := a.progressPhase % cycle
	if pos > travel {
		pos = cycle - pos // bounce back
	}

	result := make([]rune, 0, width+2)
	result = append(result, '[')
	for i := 0; i < width; i++ {
		if i >= pos && i < pos+highlightWidth {
			result = append(result, '\u2588') // full block
		} else {
			result = append(result, '\u2591') // light shade
		}
	}
	result = append(result, ']')
	return string(result)
}

func (a *App) drawBreadcrumb() {
	style := tcell.StyleDefault.
		Background(tcell.NewRGBColor(40, 44, 52)).
		Foreground(tcell.NewRGBColor(171, 178, 191))

	// Fill background
	for x := 0; x < a.width; x++ {
		a.screen.SetContent(x, 0, ' ', nil, style)
	}

	// Build breadcrumb path
	var parts []string
	node := a.focus
	for node != nil {
		parts = append([]string{node.Name}, parts...)
		node = node.Parent
	}

	// Size label on the right
	sizeStr := format.Size(a.focus.Size)
	sizeRuneLen := utf8.RuneCountInString(sizeStr)
	availWidth := a.width - sizeRuneLen - 3 // 1 left pad + 1 right pad + 1 gap

	// Collapse breadcrumb if too long
	sep := " \u203A "
	sepLen := utf8.RuneCountInString(sep)
	crumb := collapseBreadcrumb(parts, sep, sepLen, availWidth)

	x := 1
	for i, part := range crumb {
		if i > 0 {
			drawStr(a.screen, x, 0, style.Foreground(tcell.NewRGBColor(92, 99, 112)), sep)
			x += sepLen
		}
		s := style
		if i == len(crumb)-1 {
			s = s.Foreground(tcell.NewRGBColor(86, 182, 194)).Bold(true)
		}
		drawStr(a.screen, x, 0, s, part)
		x += utf8.RuneCountInString(part)
	}

	// Show total size (fixed: use rune count)
	drawStr(a.screen, a.width-sizeRuneLen-1, 0,
		style.Foreground(tcell.NewRGBColor(229, 192, 123)), sizeStr)
}

// collapseBreadcrumb collapses middle segments with ellipsis when too long.
func collapseBreadcrumb(parts []string, sep string, sepLen int, maxWidth int) []string {
	// Measure full width
	fullWidth := 0
	for i, p := range parts {
		if i > 0 {
			fullWidth += sepLen
		}
		fullWidth += utf8.RuneCountInString(p)
	}
	if fullWidth <= maxWidth {
		return parts
	}

	// Try: first + "..." + last 2
	if len(parts) > 3 {
		collapsed := []string{parts[0], "\u2026"}
		collapsed = append(collapsed, parts[len(parts)-2:]...)
		w := 0
		for i, p := range collapsed {
			if i > 0 {
				w += sepLen
			}
			w += utf8.RuneCountInString(p)
		}
		if w <= maxWidth {
			return collapsed
		}
	}

	// Fallback: "..." + last segment
	last := parts[len(parts)-1]
	fallback := []string{"\u2026", last}
	w := utf8.RuneCountInString("\u2026") + sepLen + utf8.RuneCountInString(last)
	if w <= maxWidth {
		return fallback
	}

	// Absolute fallback: just truncate last segment
	return []string{truncateWithEllipsis(last, maxWidth)}
}

func (a *App) drawTreemap() {
	if len(a.cells) == 0 {
		return
	}

	tmW := a.width - sidebarWidth
	if tmW < 10 {
		tmW = a.width
	}

	// Find max size for relative brightness
	var maxSize int64
	for _, c := range a.cells {
		if c.Node.Size > maxSize {
			maxSize = c.Node.Size
		}
	}

	for i, cell := range a.cells {
		r := cell.Rect
		baseColor := ColorFor(cell.Node.Name, cell.Node.IsDir)

		// Dim smaller files (raised brightness floor)
		brightness := 0.65 + 0.35*(float64(cell.Node.Size)/float64(maxSize))
		if brightness > 1.0 {
			brightness = 1.0
		}
		bg := DimColor(baseColor, brightness)
		fg := ContrastFg(bg)

		isCursor := i == a.cursor
		if isCursor {
			bg = baseColor // full brightness, no dimming
			fg = ContrastFg(bg)
		}

		style := tcell.StyleDefault.Background(bg).Foreground(fg)

		// Fill cell
		for dy := 0; dy < r.H; dy++ {
			for dx := 0; dx < r.W; dx++ {
				a.screen.SetContent(r.X+dx, r.Y+dy+breadcrumbH, ' ', nil, style)
			}
		}

		// Draw border (1px darker line at right and bottom edges)
		if !isCursor {
			borderStyle := tcell.StyleDefault.Background(DimColor(bg, 0.5)).Foreground(fg)
			// Right edge
			if r.W > 1 {
				for dy := 0; dy < r.H; dy++ {
					a.screen.SetContent(r.X+r.W-1, r.Y+dy+breadcrumbH, ' ', nil, borderStyle)
				}
			}
			// Bottom edge
			if r.H > 1 {
				for dx := 0; dx < r.W; dx++ {
					a.screen.SetContent(r.X+dx, r.Y+r.H-1+breadcrumbH, ' ', nil, borderStyle)
				}
			}
		} else {
			// Cursor border: gold/amber outline
			cursorBorder := tcell.StyleDefault.
				Background(tcell.NewRGBColor(229, 192, 123)).
				Foreground(tcell.NewRGBColor(20, 20, 20))
			for dx := 0; dx < r.W; dx++ {
				a.screen.SetContent(r.X+dx, r.Y+breadcrumbH, '\u2580', nil, cursorBorder)
				a.screen.SetContent(r.X+dx, r.Y+r.H-1+breadcrumbH, '\u2584', nil, cursorBorder)
			}
			for dy := 0; dy < r.H; dy++ {
				a.screen.SetContent(r.X, r.Y+dy+breadcrumbH, '\u2590', nil, cursorBorder)
				a.screen.SetContent(r.X+r.W-1, r.Y+dy+breadcrumbH, '\u258C', nil, cursorBorder)
			}
		}

		// Label: filename + size if cell is big enough
		if r.W >= 4 && r.H >= 1 {
			label := cell.Node.Name
			if utf8.RuneCountInString(label) > r.W-2 {
				label = truncateWithEllipsis(label, r.W-2)
			}
			labelStyle := style.Bold(true)
			if isCursor {
				labelStyle = tcell.StyleDefault.
					Background(bg).
					Foreground(fg).Bold(true)
			}
			// Fix: when H==1, draw at r.Y+breadcrumbH (not r.Y+1+breadcrumbH)
			labelY := r.Y + 1 + breadcrumbH
			if r.H == 1 {
				labelY = r.Y + breadcrumbH
			}
			drawStr(a.screen, r.X+1, labelY, labelStyle, label)

			if r.H >= 3 && r.W >= 6 {
				sizeLabel := format.Size(cell.Node.Size)
				if utf8.RuneCountInString(sizeLabel) <= r.W-2 {
					sizeStyle := style
					if isCursor {
						sizeStyle = labelStyle.Bold(false)
					}
					drawStr(a.screen, r.X+1, r.Y+2+breadcrumbH, sizeStyle, sizeLabel)
				}
			}
		}
	}
}

func (a *App) drawSidebar() {
	tmW := a.width - sidebarWidth
	if tmW < 10 {
		return // No room for sidebar
	}

	x0 := tmW
	bgStyle := tcell.StyleDefault.
		Background(tcell.NewRGBColor(33, 37, 43)).
		Foreground(tcell.NewRGBColor(171, 178, 191))

	// Fill sidebar background
	for y := 0; y < a.height; y++ {
		for x := x0; x < a.width; x++ {
			a.screen.SetContent(x, y, ' ', nil, bgStyle)
		}
	}

	// Separator line
	sepStyle := tcell.StyleDefault.
		Background(tcell.NewRGBColor(50, 55, 65)).
		Foreground(tcell.NewRGBColor(50, 55, 65))
	for y := 0; y < a.height; y++ {
		a.screen.SetContent(x0, y, '\u2502', nil, sepStyle)
	}

	if len(a.cells) == 0 || a.cursor >= len(a.cells) {
		return
	}

	node := a.cells[a.cursor].Node
	y := breadcrumbH + 1
	pad := x0 + 2
	w := sidebarWidth - 3

	labelStyle := bgStyle.Foreground(tcell.NewRGBColor(92, 99, 112))
	valueStyle := bgStyle.Foreground(tcell.NewRGBColor(171, 178, 191))
	titleStyle := bgStyle.Foreground(tcell.NewRGBColor(86, 182, 194)).Bold(true)
	dimDivStyle := bgStyle.Foreground(tcell.NewRGBColor(62, 68, 81))

	// Name (title)
	name := node.Name
	if utf8.RuneCountInString(name) > w {
		name = truncateWithEllipsis(name, w)
	}
	drawStr(a.screen, pad, y, titleStyle, name)
	y++

	// Divider
	y = drawSidebarDivider(a.screen, pad, y, w, dimDivStyle)

	// Key-value section: Type, Size, Files, Folders, % Parent
	kvLabelWidth := 10 // fixed label column width

	// Type
	cat := CategoryFor(node.Name, node.IsDir)
	catColor := categoryColors[cat]
	catLabel := categoryLabels[cat]
	drawStr(a.screen, pad, y, labelStyle, "Type")
	drawStr(a.screen, pad+kvLabelWidth, y, bgStyle.Foreground(catColor), catLabel)
	y++

	// Size
	drawStr(a.screen, pad, y, labelStyle, "Size")
	sizeVal := format.Size(node.Size)
	drawStr(a.screen, pad+kvLabelWidth, y, bgStyle.Foreground(tcell.NewRGBColor(224, 108, 117)).Bold(true), sizeVal)
	y++

	if node.IsDir {
		// Files
		drawStr(a.screen, pad, y, labelStyle, "Files")
		drawStr(a.screen, pad+kvLabelWidth, y, valueStyle, format.Count(node.FileCount))
		y++

		// Folders
		drawStr(a.screen, pad, y, labelStyle, "Folders")
		drawStr(a.screen, pad+kvLabelWidth, y, valueStyle, format.Count(node.DirCount))
		y++
	}

	// Percentage of parent
	if a.focus.Size > 0 {
		pct := float64(node.Size) / float64(a.focus.Size) * 100
		drawStr(a.screen, pad, y, labelStyle, "% Parent")
		drawStr(a.screen, pad+kvLabelWidth, y, valueStyle, fmt.Sprintf("%.1f%%", pct))
		y++
	}

	// Divider
	y = drawSidebarDivider(a.screen, pad, y, w, dimDivStyle)

	// Path
	drawStr(a.screen, pad, y, labelStyle, "Path")
	y++
	path := node.Path()
	pathRunes := []rune(path)
	for len(pathRunes) > 0 {
		chunkLen := w
		if chunkLen > len(pathRunes) {
			chunkLen = len(pathRunes)
		}
		chunk := string(pathRunes[:chunkLen])
		drawStr(a.screen, pad, y, bgStyle.Foreground(tcell.NewRGBColor(130, 137, 151)), chunk)
		pathRunes = pathRunes[chunkLen:]
		y++
	}

	// Divider
	y = drawSidebarDivider(a.screen, pad, y, w, dimDivStyle)

	// Top children (if directory)
	if node.IsDir && len(node.Children) > 0 {
		drawStr(a.screen, pad, y, titleStyle, "Top Items")
		y++
		maxItems := 10
		if y+maxItems > a.height-2 {
			maxItems = a.height - 2 - y
		}
		for i := 0; i < maxItems && i < len(node.Children); i++ {
			child := node.Children[i]
			cColor := ColorFor(child.Name, child.IsDir)
			childName := child.Name
			size := format.Size(child.Size)
			sizeRuneLen := utf8.RuneCountInString(size)
			maxName := w - sizeRuneLen - 3
			if maxName < 4 {
				maxName = 4
			}
			if utf8.RuneCountInString(childName) > maxName {
				childName = truncateWithEllipsis(childName, maxName)
			}
			drawStr(a.screen, pad, y, bgStyle.Foreground(cColor), "\u25AA "+childName)
			drawStr(a.screen, pad+w-sizeRuneLen, y, bgStyle.Foreground(tcell.NewRGBColor(171, 178, 191)), size)
			y++
		}
	}
}

// drawSidebarDivider draws a horizontal dim divider and returns the next y.
func drawSidebarDivider(screen tcell.Screen, x, y, w int, style tcell.Style) int {
	for dx := 0; dx < w; dx++ {
		screen.SetContent(x+dx, y, '\u2500', nil, style)
	}
	return y + 1
}

func (a *App) drawStatusBar() {
	y := a.height - 1
	style := tcell.StyleDefault.
		Background(tcell.NewRGBColor(40, 44, 52)).
		Foreground(tcell.NewRGBColor(130, 137, 151))

	for x := 0; x < a.width; x++ {
		a.screen.SetContent(x, y, ' ', nil, style)
	}

	left := fmt.Sprintf(" %s  %s files  %s dirs",
		format.Size(a.focus.Size),
		format.Count(a.focus.FileCount),
		format.Count(a.focus.DirCount))
	drawStr(a.screen, 0, y, style, left)

	right := "arrows:move  Enter:open  Bksp:back  ?:help  q:quit "
	drawStr(a.screen, a.width-utf8.RuneCountInString(right), y, style, right)
}

func (a *App) drawHelp() {
	// Draw dim background scrim
	scrimStyle := tcell.StyleDefault.
		Background(tcell.NewRGBColor(20, 22, 26)).
		Foreground(tcell.NewRGBColor(92, 99, 112))
	for y := 0; y < a.height; y++ {
		for x := 0; x < a.width; x++ {
			a.screen.SetContent(x, y, ' ', nil, scrimStyle)
		}
	}

	// Build help content dynamically
	type helpEntry struct {
		key  string
		desc string
	}
	type helpSection struct {
		title   string
		entries []helpEntry
	}

	sections := []helpSection{
		{
			title: "Navigation",
			entries: []helpEntry{
				{"\u2191\u2193 / j k", "Navigate cells"},
				{"\u2190\u2192 / h l", "Spatial move"},
				{"Enter / l", "Drill into dir"},
				{"Bksp / h", "Go back"},
			},
		},
		{
			title: "Actions",
			entries: []helpEntry{
				{"Esc", "Back / quit"},
			},
		},
		{
			title: "General",
			entries: []helpEntry{
				{"?", "Toggle help"},
				{"q / Ctrl+C", "Quit"},
			},
		},
	}

	// Calculate box dimensions
	boxTitle := "LINTREE HELP"
	innerWidth := 34
	keyColWidth := 14

	// Count total lines: title + sections (title + entries + blank) + footer
	totalLines := 1 // box title
	for _, sec := range sections {
		totalLines += 1 + len(sec.entries) + 1 // section title + entries + blank
	}
	totalLines += 1 // footer "Press any key to close"
	boxH := totalLines + 2 // +2 for top/bottom border
	boxW := innerWidth + 4 // +4 for borders + padding

	startX := (a.width - boxW) / 2
	startY := (a.height - boxH) / 2
	if startY < 0 {
		startY = 0
	}

	borderStyle := tcell.StyleDefault.
		Background(tcell.NewRGBColor(33, 37, 43)).
		Foreground(tcell.NewRGBColor(86, 182, 194))
	textStyle := tcell.StyleDefault.
		Background(tcell.NewRGBColor(33, 37, 43)).
		Foreground(tcell.NewRGBColor(171, 178, 191))
	dimTextStyle := tcell.StyleDefault.
		Background(tcell.NewRGBColor(33, 37, 43)).
		Foreground(tcell.NewRGBColor(92, 99, 112))
	headingStyle := tcell.StyleDefault.
		Background(tcell.NewRGBColor(33, 37, 43)).
		Foreground(tcell.NewRGBColor(229, 192, 123)).Bold(true)
	keyStyle := tcell.StyleDefault.
		Background(tcell.NewRGBColor(33, 37, 43)).
		Foreground(tcell.NewRGBColor(86, 182, 194))

	// Fill box background
	for dy := 0; dy < boxH; dy++ {
		for dx := 0; dx < boxW; dx++ {
			a.screen.SetContent(startX+dx, startY+dy, ' ', nil, textStyle)
		}
	}

	// Top border
	a.screen.SetContent(startX, startY, '\u2554', nil, borderStyle)
	for dx := 1; dx < boxW-1; dx++ {
		a.screen.SetContent(startX+dx, startY, '\u2550', nil, borderStyle)
	}
	a.screen.SetContent(startX+boxW-1, startY, '\u2557', nil, borderStyle)

	// Bottom border
	a.screen.SetContent(startX, startY+boxH-1, '\u255A', nil, borderStyle)
	for dx := 1; dx < boxW-1; dx++ {
		a.screen.SetContent(startX+dx, startY+boxH-1, '\u2550', nil, borderStyle)
	}
	a.screen.SetContent(startX+boxW-1, startY+boxH-1, '\u255D', nil, borderStyle)

	// Side borders
	for dy := 1; dy < boxH-1; dy++ {
		a.screen.SetContent(startX, startY+dy, '\u2551', nil, borderStyle)
		a.screen.SetContent(startX+boxW-1, startY+dy, '\u2551', nil, borderStyle)
	}

	// Title line
	cy := startY + 1
	titlePad := (innerWidth - utf8.RuneCountInString(boxTitle)) / 2
	drawStr(a.screen, startX+2+titlePad, cy, headingStyle, boxTitle)
	cy++

	// Sections
	for _, sec := range sections {
		// Section separator
		a.screen.SetContent(startX, cy, '\u2560', nil, borderStyle)
		for dx := 1; dx < boxW-1; dx++ {
			a.screen.SetContent(startX+dx, cy, '\u2550', nil, borderStyle)
		}
		a.screen.SetContent(startX+boxW-1, cy, '\u2563', nil, borderStyle)
		cy++

		// Section title
		drawStr(a.screen, startX+2, cy, headingStyle, sec.title)
		cy++

		// Entries
		for _, e := range sec.entries {
			drawStr(a.screen, startX+2, cy, keyStyle, e.key)
			drawStr(a.screen, startX+2+keyColWidth, cy, textStyle, e.desc)
			cy++
		}
	}

	// Footer: "Press any key to close"
	footer := "Press any key to close"
	footerPad := (innerWidth - utf8.RuneCountInString(footer)) / 2
	drawStr(a.screen, startX+2+footerPad, cy, dimTextStyle, footer)
}

func drawStr(s tcell.Screen, x, y int, style tcell.Style, str string) {
	for _, r := range str {
		s.SetContent(x, y, r, nil, style)
		x++
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// truncateRunes truncates a string to at most n runes, safely handling multi-byte UTF-8.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// truncateWithEllipsis truncates a string to at most maxWidth runes,
// appending a single ellipsis character (U+2026) when truncated.
func truncateWithEllipsis(s string, maxWidth int) string {
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth <= 1 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-1]) + "\u2026"
}


