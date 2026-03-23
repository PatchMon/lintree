package scanner

import "testing"

func TestSortChildren(t *testing.T) {
	root := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{Name: "small", Size: 10},
			{Name: "large", Size: 1000},
			{Name: "medium", Size: 500},
		},
	}
	root.SortChildren()

	for i := 1; i < len(root.Children); i++ {
		if root.Children[i].Size > root.Children[i-1].Size {
			t.Errorf("children not sorted descending at index %d: %d > %d",
				i, root.Children[i].Size, root.Children[i-1].Size)
		}
	}
	if root.Children[0].Name != "large" {
		t.Errorf("expected first child 'large', got %q", root.Children[0].Name)
	}
}

func TestSortChildrenRecursive(t *testing.T) {
	child := &FileNode{
		Name:  "dir",
		IsDir: true,
		Children: []*FileNode{
			{Name: "b", Size: 1},
			{Name: "a", Size: 100},
		},
	}
	root := &FileNode{
		Name:     "root",
		IsDir:    true,
		Children: []*FileNode{child},
	}
	root.SortChildren()

	if child.Children[0].Name != "a" {
		t.Errorf("recursive sort failed: expected first grandchild 'a', got %q", child.Children[0].Name)
	}
}

func TestSortChildrenNil(t *testing.T) {
	var n *FileNode
	n.SortChildren() // should not panic
}

func TestTopChildrenUnderLimit(t *testing.T) {
	root := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{Name: "a", Size: 300},
			{Name: "b", Size: 200},
		},
	}

	result := root.TopChildren(5)
	if len(result) != 2 {
		t.Errorf("expected 2 children, got %d", len(result))
	}
}

func TestTopChildrenExactLimit(t *testing.T) {
	root := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{Name: "a", Size: 300},
			{Name: "b", Size: 200},
			{Name: "c", Size: 100},
		},
	}

	result := root.TopChildren(3)
	if len(result) != 3 {
		t.Errorf("expected 3 children, got %d", len(result))
	}
}

func TestTopChildrenWithOther(t *testing.T) {
	root := &FileNode{
		Name:  "root",
		IsDir: true,
		Children: []*FileNode{
			{Name: "a", Size: 500},
			{Name: "b", Size: 400},
			{Name: "c", Size: 100},
			{Name: "d", Size: 50},
			{Name: "e", Size: 25},
		},
	}

	result := root.TopChildren(2)
	if len(result) != 3 {
		t.Fatalf("expected 3 (2 top + 1 other), got %d", len(result))
	}

	other := result[2]
	if other.Name != "(3 other)" {
		t.Errorf("other node name = %q, want %q", other.Name, "(3 other)")
	}
	wantSize := int64(100 + 50 + 25)
	if other.Size != wantSize {
		t.Errorf("other node size = %d, want %d", other.Size, wantSize)
	}
	if !other.IsDir {
		t.Error("other node should be marked as directory")
	}
}

func TestComputeSizes(t *testing.T) {
	leaf1 := &FileNode{Name: "a.txt", Size: 100}
	leaf2 := &FileNode{Name: "b.txt", Size: 200}
	subdir := &FileNode{
		Name:  "sub",
		IsDir: true,
		Children: []*FileNode{
			{Name: "c.txt", Size: 50},
		},
	}
	root := &FileNode{
		Name:     "root",
		IsDir:    true,
		Children: []*FileNode{leaf1, leaf2, subdir},
	}

	root.computeSizes()

	if subdir.Size != 50 {
		t.Errorf("subdir size = %d, want 50", subdir.Size)
	}
	if root.Size != 350 {
		t.Errorf("root size = %d, want 350", root.Size)
	}
	if root.FileCount != 3 {
		t.Errorf("root file count = %d, want 3", root.FileCount)
	}
	if root.DirCount != 1 {
		t.Errorf("root dir count = %d, want 1", root.DirCount)
	}
}

func TestComputeSizesDeep(t *testing.T) {
	// root/dir1/dir2/file.txt (size=42)
	file := &FileNode{Name: "file.txt", Size: 42}
	dir2 := &FileNode{Name: "dir2", IsDir: true, Children: []*FileNode{file}}
	dir1 := &FileNode{Name: "dir1", IsDir: true, Children: []*FileNode{dir2}}
	root := &FileNode{Name: "root", IsDir: true, Children: []*FileNode{dir1}}

	root.computeSizes()

	if root.Size != 42 {
		t.Errorf("root size = %d, want 42", root.Size)
	}
	if root.FileCount != 1 {
		t.Errorf("root FileCount = %d, want 1", root.FileCount)
	}
	if root.DirCount != 2 {
		t.Errorf("root DirCount = %d, want 2", root.DirCount)
	}
}
