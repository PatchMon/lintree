package scanner

import (
	"context"
	"os"
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

// Scan walks the filesystem tree rooted at root concurrently.
func Scan(ctx context.Context, root string) (<-chan Progress, <-chan *FileNode, <-chan error) {
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
			Name:  filepath.Base(absRoot),
			Path:  absRoot,
			IsDir: true,
		}

		var dirsScanned, filesFound, bytesFound atomic.Int64
		var currentPath atomic.Value
		currentPath.Store(absRoot)

		// Progress reporter goroutine
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(50 * time.Millisecond)
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

		// Bounded worker pool: semaphore gates goroutine creation, not just ReadDir
		workers := min(runtime.NumCPU()*2, 32)
		sem := make(chan struct{}, workers)

		var wg sync.WaitGroup
		scanDir(ctx, rootNode, sem, &wg, &dirsScanned, &filesFound, &bytesFound, &currentPath)
		wg.Wait()
		close(done)

		// Handle cancellation: send partial result or error
		if ctx.Err() != nil {
			errCh <- ctx.Err()
			return
		}

		// Bottom-up size computation (single-threaded, after all scanning done)
		rootNode.computeSizes()
		rootNode.SortChildren()
		resultCh <- rootNode
	}()

	return progCh, resultCh, errCh
}

func scanDir(ctx context.Context, node *FileNode, sem chan struct{}, wg *sync.WaitGroup, dirsScanned, filesFound, bytesFound *atomic.Int64, currentPath *atomic.Value) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	if skipDirs[node.Path] {
		return
	}

	sem <- struct{}{}
	entries, err := os.ReadDir(node.Path)
	<-sem

	if err != nil {
		node.Err = err
		return
	}

	dirsScanned.Add(1)
	currentPath.Store(node.Path)

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Skip symlinks to prevent infinite loops and double-counting
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		childPath := filepath.Join(node.Path, entry.Name())
		child := &FileNode{
			Name:   entry.Name(),
			Path:   childPath,
			IsDir:  entry.IsDir(),
			Parent: node,
		}

		if entry.IsDir() {
			node.addChild(child)
			wg.Add(1)
			go func() {
				defer wg.Done()
				scanDir(ctx, child, sem, wg, dirsScanned, filesFound, bytesFound, currentPath)
			}()
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
