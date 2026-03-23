package ui

import (
	"context"
	"fmt"
	"lintree/internal/format"
	"lintree/internal/scanner"
	"lintree/internal/treemap"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

const (
	sidebarWidth   = 30
	breadcrumbH    = 1
	statusBarH     = 1
	maxVisibleCells = 200
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
}

// Run starts the TUI application.
func Run(rootPath string) error {
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
	}

	// Start scan
	progCh, resultCh, errCh := scanner.Scan(ctx, rootPath)

	// Main event loop
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

	for {
		// Draw
		app.draw()

		select {
		case ev := <-eventCh:
			switch e := ev.(type) {
			case *tcell.EventResize:
				app.width, app.height = e.Size()
				screen.Sync()
				app.rebuildLayout()
			case *tcell.EventKey:
				if app.handleKey(e) {
					return nil
				}
			case *tcell.EventMouse:
				app.handleMouse(e)
			}

		case p, ok := <-progCh:
			if ok {
				app.progress = p
			}

		case result, ok := <-resultCh:
			if ok && result != nil {
				app.root = result
				app.focus = result
				app.scanning = false
				app.rebuildLayout()
			}

		case scanErr, ok := <-errCh:
			if ok && scanErr != nil {
				screen.Fini()
				return fmt.Errorf("scan error: %w", scanErr)
			}
		}
	}
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
	style := tcell.StyleDefault.Foreground(tcell.NewRGBColor(86, 182, 194))
	line1 := fmt.Sprintf("  Scanning filesystem...")
	line2 := fmt.Sprintf("  Dirs: %d  Files: %d  Size: %s",
		a.progress.DirsScanned, a.progress.FilesFound,
		format.Size(a.progress.BytesFound))

	path := a.progress.CurrentPath
	maxW := a.width - 4
	if len(path) > maxW {
		path = "..." + path[len(path)-maxW+3:]
	}
	line3 := fmt.Sprintf("  %s", path)

	// Spinner
	spinChars := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
	spinIdx := int(a.progress.DirsScanned) % len(spinChars)
	spinner := string(spinChars[spinIdx])

	y := a.height / 2
	drawStr(a.screen, 2, y-1, style.Bold(true), spinner+" "+line1)
	drawStr(a.screen, 2, y, style, line2)
	drawStr(a.screen, 2, y+1, tcell.StyleDefault.Foreground(tcell.NewRGBColor(92, 99, 112)), line3)
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

	sep := " › "
	x := 1
	for i, part := range parts {
		if i > 0 {
			drawStr(a.screen, x, 0, style.Foreground(tcell.NewRGBColor(92, 99, 112)), sep)
			x += utf8.RuneCountInString(sep)
		}
		s := style
		if i == len(parts)-1 {
			s = s.Foreground(tcell.NewRGBColor(86, 182, 194)).Bold(true)
		}
		drawStr(a.screen, x, 0, s, part)
		x += utf8.RuneCountInString(part)
	}

	// Show total size
	sizeStr := format.Size(a.focus.Size)
	drawStr(a.screen, a.width-len(sizeStr)-1, 0,
		style.Foreground(tcell.NewRGBColor(229, 192, 123)), sizeStr)
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

		// Dim smaller files
		brightness := 0.5 + 0.5*(float64(cell.Node.Size)/float64(maxSize))
		if brightness > 1.0 {
			brightness = 1.0
		}
		bg := DimColor(baseColor, brightness)
		fg := ContrastFg(bg)

		isCursor := i == a.cursor
		if isCursor {
			bg = tcell.NewRGBColor(255, 255, 255)
			fg = tcell.NewRGBColor(20, 20, 20)
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
			// Cursor border: bright outline
			cursorBorder := tcell.StyleDefault.
				Background(tcell.NewRGBColor(255, 200, 50)).
				Foreground(tcell.NewRGBColor(20, 20, 20))
			for dx := 0; dx < r.W; dx++ {
				a.screen.SetContent(r.X+dx, r.Y+breadcrumbH, '▀', nil, cursorBorder)
				a.screen.SetContent(r.X+dx, r.Y+r.H-1+breadcrumbH, '▄', nil, cursorBorder)
			}
			for dy := 0; dy < r.H; dy++ {
				a.screen.SetContent(r.X, r.Y+dy+breadcrumbH, '▐', nil, cursorBorder)
				a.screen.SetContent(r.X+r.W-1, r.Y+dy+breadcrumbH, '▌', nil, cursorBorder)
			}
		}

		// Label: filename + size if cell is big enough
		if r.W >= 4 && r.H >= 1 {
			label := cell.Node.Name
			if utf8.RuneCountInString(label) > r.W-2 {
				label = truncateRunes(label, r.W-2)
			}
			labelStyle := style.Bold(true)
			if isCursor {
				labelStyle = tcell.StyleDefault.
					Background(tcell.NewRGBColor(255, 255, 255)).
					Foreground(tcell.NewRGBColor(20, 20, 20)).Bold(true)
			}
			drawStr(a.screen, r.X+1, r.Y+1+breadcrumbH, labelStyle, label)

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
		a.screen.SetContent(x0, y, '│', nil, sepStyle)
	}

	if len(a.cells) == 0 || a.cursor >= len(a.cells) {
		return
	}

	node := a.cells[a.cursor].Node
	y := breadcrumbH + 1
	pad := x0 + 2
	w := sidebarWidth - 3

	// Title
	titleStyle := bgStyle.Foreground(tcell.NewRGBColor(86, 182, 194)).Bold(true)
	name := node.Name
	if utf8.RuneCountInString(name) > w {
		name = truncateRunes(name, w)
	}
	drawStr(a.screen, pad, y, titleStyle, name)
	y += 2

	// File type icon
	cat := CategoryFor(node.Name, node.IsDir)
	catColor := categoryColors[cat]
	catLabel := categoryLabels[cat]
	catStyle := bgStyle.Foreground(catColor)
	drawStr(a.screen, pad, y, catStyle, "● "+catLabel)
	y += 2

	// Size
	labelStyle := bgStyle.Foreground(tcell.NewRGBColor(92, 99, 112))
	valueStyle := bgStyle.Foreground(tcell.NewRGBColor(224, 108, 117)).Bold(true)
	drawStr(a.screen, pad, y, labelStyle, "Size")
	drawStr(a.screen, pad, y+1, valueStyle, format.Size(node.Size))
	y += 3

	if node.IsDir {
		drawStr(a.screen, pad, y, labelStyle, "Files")
		drawStr(a.screen, pad, y+1, bgStyle, fmt.Sprintf("%d", node.FileCount))
		y += 3

		drawStr(a.screen, pad, y, labelStyle, "Folders")
		drawStr(a.screen, pad, y+1, bgStyle, fmt.Sprintf("%d", node.DirCount))
		y += 3
	}

	// Percentage of parent
	if a.focus.Size > 0 {
		pct := float64(node.Size) / float64(a.focus.Size) * 100
		drawStr(a.screen, pad, y, labelStyle, "% of parent")
		drawStr(a.screen, pad, y+1, bgStyle, fmt.Sprintf("%.1f%%", pct))
		y += 3
	}

	// Path
	drawStr(a.screen, pad, y, labelStyle, "Path")
	y++
	path := node.Path
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
	y += 1

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
			name := child.Name
			size := format.Size(child.Size)
			sizeRuneLen := utf8.RuneCountInString(size)
			maxName := w - sizeRuneLen - 3
			if maxName < 4 {
				maxName = 4
			}
			if utf8.RuneCountInString(name) > maxName {
				name = truncateRunes(name, maxName-1) + "…"
			}
			drawStr(a.screen, pad, y, bgStyle.Foreground(cColor), "▪ "+name)
			drawStr(a.screen, pad+w-sizeRuneLen, y, bgStyle.Foreground(tcell.NewRGBColor(171, 178, 191)), size)
			y++
		}
	}
}

func (a *App) drawStatusBar() {
	y := a.height - 1
	style := tcell.StyleDefault.
		Background(tcell.NewRGBColor(40, 44, 52)).
		Foreground(tcell.NewRGBColor(130, 137, 151))

	for x := 0; x < a.width; x++ {
		a.screen.SetContent(x, y, ' ', nil, style)
	}

	left := fmt.Sprintf(" %s  │  %d files  │  %d folders",
		format.Size(a.focus.Size), a.focus.FileCount, a.focus.DirCount)
	drawStr(a.screen, 0, y, style, left)

	right := "arrows/hjkl:navigate  Enter/l:open  Bksp/h:back  ?:help  q:quit "
	drawStr(a.screen, a.width-utf8.RuneCountInString(right), y, style, right)
}

func (a *App) drawHelp() {
	lines := []string{
		"            ╔══════════════════════════════╗",
		"            ║        LINTREE HELP          ║",
		"            ╠══════════════════════════════╣",
		"            ║  ↑↓ / j k    Navigate cells  ║",
		"            ║  ←→ / h l    Spatial move     ║",
		"            ║  Enter / l   Drill into dir   ║",
		"            ║  Bksp / h    Go back           ║",
		"            ║  Esc         Back / quit       ║",
		"            ║  ?           Toggle help       ║",
		"            ║  q / Ctrl+C  Quit              ║",
		"            ╚══════════════════════════════╝",
	}
	startY := (a.height - len(lines)) / 2
	bg := tcell.StyleDefault.
		Background(tcell.NewRGBColor(33, 37, 43)).
		Foreground(tcell.NewRGBColor(171, 178, 191))

	for i, line := range lines {
		drawStr(a.screen, (a.width-utf8.RuneCountInString(line))/2, startY+i, bg, line)
	}
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

