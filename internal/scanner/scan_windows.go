//go:build windows

package scanner

import (
	"context"
	"os"
	"path/filepath"
)

func processDir(ctx context.Context, node *FileNode, dirPath string, st *scanState, pathBuf []byte) {
	defer st.wg.Done()

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

	st.dirsScanned.Add(1)
	st.currentPath.Store(dirPath)

	if cap(node.Children) == 0 {
		node.Children = make([]*FileNode, 0, len(entries))
	}

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
			st.wg.Add(1)
			select {
			case st.workCh <- work{node: child, path: childPath}:
			default:
				processDir(ctx, child, childPath, st, pathBuf)
			}
		} else {
			if info, err := entry.Info(); err == nil {
				child.Size = fileSize(info)
			} else {
				child.Err = err
			}
			st.filesFound.Add(1)
			st.bytesFound.Add(child.Size)
			node.addChild(child)
		}
	}
}
