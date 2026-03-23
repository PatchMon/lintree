package treemap

import "lintree/internal/scanner"

// Rect represents a rectangle in terminal cell coordinates.
type Rect struct {
	X, Y, W, H int
}

// Area returns the area in cells.
func (r Rect) Area() int {
	return r.W * r.H
}

// Cell maps a FileNode to a screen rectangle.
type Cell struct {
	Node  *scanner.FileNode
	Rect  Rect
	Depth int
}
