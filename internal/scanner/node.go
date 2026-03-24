package scanner

import (
	"fmt"
	"sort"
	"strings"
)

// FileNode represents a file or directory in the scanned tree.
type FileNode struct {
	Name      string
	Size      int64 // Total size including children
	IsDir     bool
	Children  []*FileNode
	Parent    *FileNode
	FileCount int64
	DirCount  int64
	Err error
}

// Path computes the full path by walking up the parent chain.
// Only call this when you actually need the path (sidebar display, etc).
func (n *FileNode) Path() string {
	if n.Parent == nil {
		return n.Name
	}
	// Count depth to pre-allocate
	depth := 0
	for p := n; p != nil; p = p.Parent {
		depth++
	}
	parts := make([]string, depth)
	node := n
	for i := depth - 1; i >= 0; i-- {
		parts[i] = node.Name
		node = node.Parent
	}
	// The root name is an absolute path like "/home/user", so Join handles it.
	return strings.Join(parts, "/")
}

// addChild appends a child node. Safe because each directory is processed
// by exactly one goroutine.
func (n *FileNode) addChild(child *FileNode) {
	n.Children = append(n.Children, child)
}

// computeSizes walks bottom-up to compute aggregate Size/FileCount/DirCount.
// Call this after scanning is complete (no concurrency needed).
func (n *FileNode) computeSizes() {
	if !n.IsDir {
		n.FileCount = 0
		n.DirCount = 0
		return
	}
	var totalSize int64
	var files, dirs int64
	for _, c := range n.Children {
		c.computeSizes()
		totalSize += c.Size
		if c.IsDir {
			dirs += 1 + c.DirCount
			files += c.FileCount
		} else {
			files++
		}
	}
	n.Size = totalSize
	n.FileCount = files
	n.DirCount = dirs
}

// SortChildren sorts children descending by size recursively.
func (n *FileNode) SortChildren() {
	if n == nil {
		return
	}
	sort.Slice(n.Children, func(i, j int) bool {
		return n.Children[i].Size > n.Children[j].Size
	})
	for _, c := range n.Children {
		c.SortChildren()
	}
}

// TopChildren returns the top n children by size plus an aggregate "other" node.
func (n *FileNode) TopChildren(limit int) []*FileNode {
	if len(n.Children) <= limit {
		return n.Children
	}
	top := make([]*FileNode, limit, limit+1)
	copy(top, n.Children[:limit])

	var otherSize int64
	var otherFiles, otherDirs int64
	for _, c := range n.Children[limit:] {
		otherSize += c.Size
		otherFiles += c.FileCount
		otherDirs += c.DirCount
		if !c.IsDir {
			otherFiles++
		} else {
			otherDirs++
		}
	}
	if otherSize > 0 {
		top = append(top, &FileNode{
			Name:      fmt.Sprintf("(%d other)", len(n.Children)-limit),
			Size:      otherSize,
			IsDir:     true,
			FileCount: otherFiles,
			DirCount:  otherDirs,
			Parent:    n,
		})
	}
	return top
}
