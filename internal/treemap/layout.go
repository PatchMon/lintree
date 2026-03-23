package treemap

import (
	"lintree/internal/scanner"
	"math"
)

// CellAspect corrects for terminal cells being ~2x taller than wide.
const CellAspect = 2.0

// Layout computes a squarified treemap layout for the given nodes within bounds.
func Layout(nodes []*scanner.FileNode, bounds Rect, depth int) []Cell {
	if bounds.W < 1 || bounds.H < 1 || len(nodes) == 0 {
		return nil
	}

	var totalSize int64
	for _, n := range nodes {
		if n.Size > 0 {
			totalSize += n.Size
		}
	}
	if totalSize == 0 {
		return nil
	}

	// Filter to nodes with size > 0
	filtered := make([]*scanner.FileNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Size > 0 {
			filtered = append(filtered, n)
		}
	}

	return squarify(filtered, totalSize, bounds, depth)
}

func squarify(nodes []*scanner.FileNode, totalSize int64, bounds Rect, depth int) []Cell {
	var cells []Cell
	remaining := bounds
	remainingSize := totalSize

	i := 0
	for i < len(nodes) && remaining.W > 0 && remaining.H > 0 {
		// Build a row
		row := []rowItem{{node: nodes[i], size: nodes[i].Size}}
		rowSize := nodes[i].Size
		i++

		for i < len(nodes) {
			candidate := nodes[i]
			// Clone row before append to avoid corrupting the backing array
			newRow := make([]rowItem, len(row)+1)
			copy(newRow, row)
			newRow[len(row)] = rowItem{node: candidate, size: candidate.Size}
			newRowSize := rowSize + candidate.Size

			if worstRatio(row, rowSize, remaining, remainingSize) >=
				worstRatio(newRow, newRowSize, remaining, remainingSize) {
				row = newRow
				rowSize = newRowSize
				i++
			} else {
				break
			}
		}

		// Lay out this row
		rowCells, consumed := layoutRow(row, rowSize, remaining, remainingSize, depth)
		cells = append(cells, rowCells...)

		// Shrink remaining area
		effectiveW := float64(remaining.W) * CellAspect
		effectiveH := float64(remaining.H)
		if effectiveW > effectiveH {
			remaining.X += consumed
			remaining.W -= consumed
		} else {
			remaining.Y += consumed
			remaining.H -= consumed
		}
		remainingSize -= rowSize
	}

	return cells
}

type rowItem struct {
	node *scanner.FileNode
	size int64
}

func worstRatio(row []rowItem, rowSize int64, bounds Rect, totalSize int64) float64 {
	if len(row) == 0 || totalSize == 0 || rowSize == 0 {
		return math.MaxFloat64
	}

	totalArea := float64(bounds.W) * float64(bounds.H)
	rowArea := totalArea * float64(rowSize) / float64(totalSize)

	effectiveW := float64(bounds.W) * CellAspect
	effectiveH := float64(bounds.H)

	var stripLen float64
	if effectiveW > effectiveH {
		stripLen = effectiveH
	} else {
		stripLen = effectiveW
	}
	if stripLen == 0 {
		return math.MaxFloat64
	}

	worst := 0.0
	for _, item := range row {
		frac := float64(item.size) / float64(rowSize)
		itemArea := rowArea * frac
		if itemArea <= 0 {
			continue
		}
		// Item dimensions along the strip
		itemLen := stripLen * frac
		stripWidth := rowArea / stripLen
		if itemLen == 0 || stripWidth == 0 {
			continue
		}
		ratio := itemLen / stripWidth
		if ratio < 1 {
			ratio = 1 / ratio
		}
		if ratio > worst {
			worst = ratio
		}
	}
	return worst
}

func layoutRow(row []rowItem, rowSize int64, bounds Rect, totalSize int64, depth int) ([]Cell, int) {
	if len(row) == 0 || totalSize == 0 {
		return nil, 0
	}

	rowFrac := float64(rowSize) / float64(totalSize)

	effectiveW := float64(bounds.W) * CellAspect
	effectiveH := float64(bounds.H)
	horizontal := effectiveW > effectiveH // split direction

	var cells []Cell

	if horizontal {
		// Row occupies a vertical strip on the left
		stripW := int(math.Round(float64(bounds.W) * rowFrac))
		if stripW < 1 {
			stripW = 1
		}
		if stripW > bounds.W {
			stripW = bounds.W
		}

		y := bounds.Y
		for idx, item := range row {
			frac := float64(item.size) / float64(rowSize)
			h := int(math.Round(float64(bounds.H) * frac))
			if h < 1 {
				h = 1
			}
			// Last item takes remaining space
			if idx == len(row)-1 {
				h = bounds.H - (y - bounds.Y)
			}
			if h <= 0 {
				continue
			}
			if y-bounds.Y+h > bounds.H {
				h = bounds.H - (y - bounds.Y)
			}
			cells = append(cells, Cell{
				Node:  item.node,
				Rect:  Rect{X: bounds.X, Y: y, W: stripW, H: h},
				Depth: depth,
			})
			y += h
		}
		return cells, stripW
	}

	// Vertical: row occupies a horizontal strip on top
	stripH := int(math.Round(float64(bounds.H) * rowFrac))
	if stripH < 1 {
		stripH = 1
	}
	if stripH > bounds.H {
		stripH = bounds.H
	}

	x := bounds.X
	for idx, item := range row {
		frac := float64(item.size) / float64(rowSize)
		w := int(math.Round(float64(bounds.W) * frac))
		if w < 1 {
			w = 1
		}
		if idx == len(row)-1 {
			w = bounds.W - (x - bounds.X)
		}
		if w <= 0 {
			continue
		}
		if x-bounds.X+w > bounds.W {
			w = bounds.W - (x - bounds.X)
		}
		cells = append(cells, Cell{
			Node:  item.node,
			Rect:  Rect{X: x, Y: bounds.Y, W: w, H: stripH},
			Depth: depth,
		})
		x += w
	}
	return cells, stripH
}
