package scanner

import (
	"context"
	"path/filepath"
	"runtime"
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

// scanState holds the shared mutable state for a scan.
type scanState struct {
	workCh      chan work
	wg          sync.WaitGroup
	dirsScanned atomic.Int64
	filesFound  atomic.Int64
	bytesFound  atomic.Int64
	currentPath atomic.Value
}

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

		st := &scanState{}
		st.currentPath.Store(absRoot)

		// Progress reporter
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					cp, _ := st.currentPath.Load().(string)
					select {
					case progCh <- Progress{
						DirsScanned: st.dirsScanned.Load(),
						FilesFound:  st.filesFound.Load(),
						BytesFound:  st.bytesFound.Load(),
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

		// Scale workers to CPU count. Fast mode doubles it (saturate disk I/O on SSDs).
		workers := runtime.GOMAXPROCS(0)
		if fast {
			workers *= 2
		}
		if workers < 4 {
			workers = 4
		}
		if workers > 32 {
			workers = 32
		}
		st.workCh = make(chan work, 256)

		// Start fixed worker goroutines
		for range workers {
			go func() {
				// Each worker gets a reusable path buffer to avoid allocations.
				pathBuf := make([]byte, 0, 256)
				for w := range st.workCh {
					processDir(ctx, w.node, w.path, st, pathBuf)
				}
			}()
		}

		// Seed with root
		st.wg.Add(1)
		st.workCh <- work{node: rootNode, path: absRoot}

		// Wait for all work to finish, then close the channel to stop workers
		st.wg.Wait()
		close(st.workCh)
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
