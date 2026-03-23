package treemap

import (
	"fmt"
	"lintree/internal/scanner"
	"testing"
)

func makeNodes(sizes ...int64) []*scanner.FileNode {
	nodes := make([]*scanner.FileNode, len(sizes))
	for i, s := range sizes {
		nodes[i] = &scanner.FileNode{
			Name: fmt.Sprintf("node%d", i),
			Size: s,
		}
	}
	return nodes
}

func TestLayoutEmpty(t *testing.T) {
	tests := []struct {
		name   string
		nodes  []*scanner.FileNode
		bounds Rect
	}{
		{"nil nodes", nil, Rect{0, 0, 80, 24}},
		{"empty nodes", []*scanner.FileNode{}, Rect{0, 0, 80, 24}},
		{"zero width", makeNodes(100), Rect{0, 0, 0, 24}},
		{"zero height", makeNodes(100), Rect{0, 0, 80, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cells := Layout(tt.nodes, tt.bounds, 0)
			if cells != nil {
				t.Errorf("expected nil, got %d cells", len(cells))
			}
		})
	}
}

func TestLayoutSingleNode(t *testing.T) {
	nodes := makeNodes(1000)
	bounds := Rect{0, 0, 80, 24}
	cells := Layout(nodes, bounds, 0)

	if len(cells) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(cells))
	}
	c := cells[0]
	if c.Rect.X != bounds.X || c.Rect.Y != bounds.Y {
		t.Errorf("cell origin = (%d,%d), want (%d,%d)", c.Rect.X, c.Rect.Y, bounds.X, bounds.Y)
	}
	if c.Rect.W != bounds.W || c.Rect.H != bounds.H {
		t.Errorf("cell size = %dx%d, want %dx%d", c.Rect.W, c.Rect.H, bounds.W, bounds.H)
	}
}

func TestLayoutTwoEqualNodes(t *testing.T) {
	nodes := makeNodes(500, 500)
	bounds := Rect{0, 0, 80, 24}
	cells := Layout(nodes, bounds, 0)

	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}

	totalArea := 0
	for _, c := range cells {
		totalArea += c.Rect.Area()
	}
	boundsArea := bounds.Area()
	if totalArea != boundsArea {
		t.Errorf("total cell area = %d, want %d", totalArea, boundsArea)
	}
}

func TestLayoutZeroSizeFiltered(t *testing.T) {
	nodes := makeNodes(0, 0, 0)
	bounds := Rect{0, 0, 80, 24}
	cells := Layout(nodes, bounds, 0)

	if cells != nil {
		t.Errorf("expected nil for all-zero nodes, got %d cells", len(cells))
	}
}

func TestLayoutMixedZeroAndPositive(t *testing.T) {
	nodes := makeNodes(0, 500, 0, 300)
	bounds := Rect{0, 0, 80, 24}
	cells := Layout(nodes, bounds, 0)

	if len(cells) != 2 {
		t.Fatalf("expected 2 cells (zero-size filtered), got %d", len(cells))
	}
}

func TestLayoutSmallRect(t *testing.T) {
	nodes := makeNodes(100, 200, 300)
	bounds := Rect{0, 0, 1, 1}
	cells := Layout(nodes, bounds, 0)

	// Should not panic; may produce 1 or more cells
	if cells == nil {
		t.Fatal("expected non-nil cells for 1x1 rect with positive nodes")
	}
	for _, c := range cells {
		if c.Rect.W < 1 || c.Rect.H < 1 {
			t.Errorf("cell has invalid dimensions: %dx%d", c.Rect.W, c.Rect.H)
		}
	}
}

func TestLayoutNoCellsOutOfBounds(t *testing.T) {
	nodes := makeNodes(100, 200, 300, 150, 250)
	bounds := Rect{5, 10, 60, 20}
	cells := Layout(nodes, bounds, 0)

	for i, c := range cells {
		if c.Rect.X < bounds.X || c.Rect.Y < bounds.Y {
			t.Errorf("cell %d starts before bounds: (%d,%d) < (%d,%d)",
				i, c.Rect.X, c.Rect.Y, bounds.X, bounds.Y)
		}
		if c.Rect.X+c.Rect.W > bounds.X+bounds.W {
			t.Errorf("cell %d extends past right edge: %d > %d",
				i, c.Rect.X+c.Rect.W, bounds.X+bounds.W)
		}
		if c.Rect.Y+c.Rect.H > bounds.Y+bounds.H {
			t.Errorf("cell %d extends past bottom edge: %d > %d",
				i, c.Rect.Y+c.Rect.H, bounds.Y+bounds.H)
		}
	}
}

func TestLayoutNoOverlap(t *testing.T) {
	nodes := makeNodes(100, 200, 300, 150)
	bounds := Rect{0, 0, 40, 20}
	cells := Layout(nodes, bounds, 0)

	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			a := cells[i].Rect
			b := cells[j].Rect
			// Two rects overlap if they overlap on both axes
			xOverlap := a.X < b.X+b.W && b.X < a.X+a.W
			yOverlap := a.Y < b.Y+b.H && b.Y < a.Y+a.H
			if xOverlap && yOverlap {
				t.Errorf("cells %d and %d overlap: %+v vs %+v", i, j, a, b)
			}
		}
	}
}
