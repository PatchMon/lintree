package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Progress reports scan progress.
type Progress struct {
	DirsScanned int64
	FilesFound  int64
	BytesFound  int64
	CurrentPath string
}

// skipDirs are virtual/pseudo filesystems to skip when scanning from root.
var skipDirs = map[string]bool{
	"/proc": true, "/sys": true, "/dev": true, "/run": true,
}

// work represents a directory to scan.
type work struct {
	node *FileNode
	path string // full path for this node (computed at enqueue time, not stored on node)
}

// Scan walks the filesystem tree rooted at root using a bounded worker pool.
// Scan walks the filesystem tree rooted at root using a bounded worker pool.
// Set fast=true for more workers (higher CPU, faster scan).
func Scan(ctx context.Context, root string, fast bool) (<-chan Progress, <-chan *FileNode, <-chan error) {
	progCh := make(chan Progress, 1)
	resultCh := make(chan *FileNode, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(progCh)
		defer close(resultCh)
		defer close(errCh)

		absRoot, err := filepath.Abs(root)
		if err != nil {
			errCh <- err
			return
		}

		rootNode := &FileNode{
			Name:  absRoot,
			IsDir: true,
		}

		var dirsScanned, filesFound, bytesFound atomic.Int64
		var currentPath atomic.Value
		currentPath.Store(absRoot)

		// Progress reporter
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					cp, _ := currentPath.Load().(string)
					select {
					case progCh <- Progress{
						DirsScanned: dirsScanned.Load(),
						FilesFound:  filesFound.Load(),
						BytesFound:  bytesFound.Load(),
						CurrentPath: cp,
					}:
					default:
					}
				case <-done:
					return
				case <-ctx.Done():
					return
				}
			}
		}()

		// Default: 2 workers (low CPU). Fast mode: 8 workers (saturate disk I/O).
		workers := 2
		if fast {
			workers = 8
		}
		workCh := make(chan work, 64)
		var wg sync.WaitGroup

		// Start fixed worker goroutines
		for range workers {
			go func() {
				for w := range workCh {
					processDir(ctx, w.node, w.path, workCh, &wg, &dirsScanned, &filesFound, &bytesFound, &currentPath)
				}
			}()
		}

		// Seed with root
		wg.Add(1)
		workCh <- work{node: rootNode, path: absRoot}

		// Wait for all work to finish, then close the channel to stop workers
		wg.Wait()
		close(workCh)
		close(done)

		if ctx.Err() != nil {
			errCh <- ctx.Err()
			return
		}

		rootNode.computeSizes()
		rootNode.SortChildren()
		resultCh <- rootNode
	}()

	return progCh, resultCh, errCh
}

func processDir(ctx context.Context, node *FileNode, dirPath string, workCh chan work, wg *sync.WaitGroup, dirsScanned, filesFound, bytesFound *atomic.Int64, currentPath *atomic.Value) {
	defer wg.Done()

	select {
	case <-ctx.Done():
		return
	default:
	}

	if skipDirs[dirPath] {
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		node.Err = err
		return
	}

	dirsScanned.Add(1)
	currentPath.Store(dirPath)

	// Pre-allocate children slice to avoid repeated growth
	node.mu.Lock()
	if cap(node.Children) == 0 {
		node.Children = make([]*FileNode, 0, len(entries))
	}
	node.mu.Unlock()

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		childPath := filepath.Join(dirPath, entry.Name())
		child := &FileNode{
			Name:   entry.Name(),
			IsDir:  entry.IsDir(),
			Parent: node,
		}

		if entry.IsDir() {
			node.addChild(child)
			// Try to queue work; if channel is full, process inline to avoid deadlock
			wg.Add(1)
			select {
			case workCh <- work{node: child, path: childPath}:
				// queued successfully
			default:
				// channel full, process in this goroutine to prevent deadlock
				processDir(ctx, child, childPath, workCh, wg, dirsScanned, filesFound, bytesFound, currentPath)
			}
		} else {
			if info, err := entry.Info(); err == nil {
				child.Size = fileSize(info)
			} else {
				child.Err = err
			}
			filesFound.Add(1)
			bytesFound.Add(child.Size)
			node.addChild(child)
		}
	}
}
