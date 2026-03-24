//go:build !windows

package scanner

import (
	"context"
	"os"

	"golang.org/x/sys/unix"
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

	// Open directory fd for Fstatat — avoids kernel re-resolving the full path per file.
	dirfd, err := unix.Open(dirPath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		// Fall back to entry.Info() if we can't open the dirfd.
		processDirFallback(ctx, node, dirPath, entries, st, pathBuf)
		return
	}
	defer func() { _ = unix.Close(dirfd) }()

	// Pre-allocate children slice to avoid repeated growth.
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

		name := entry.Name()

		// Build child path using reusable buffer to avoid filepath.Join allocation.
		pathBuf = appendPath(pathBuf[:0], dirPath, name)
		childPath := string(pathBuf)

		child := &FileNode{
			Name:   name,
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
			// Use Fstatat with the directory fd — single syscall, no path re-resolution.
			var stat unix.Stat_t
			if err := unix.Fstatat(dirfd, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err == nil {
				child.Size = stat.Blocks * 512
			} else {
				child.Err = err
			}
			st.filesFound.Add(1)
			st.bytesFound.Add(child.Size)
			node.addChild(child)
		}
	}
}

// processDirFallback handles the case where we can't open a dirfd (e.g. permission issues).
func processDirFallback(ctx context.Context, node *FileNode, dirPath string, entries []os.DirEntry, st *scanState, pathBuf []byte) {
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

		name := entry.Name()
		pathBuf = appendPath(pathBuf[:0], dirPath, name)
		childPath := string(pathBuf)

		child := &FileNode{
			Name:   name,
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

// appendPath builds dirPath + "/" + name into buf without allocating.
func appendPath(buf []byte, dir, name string) []byte {
	buf = append(buf, dir...)
	buf = append(buf, '/')
	buf = append(buf, name...)
	return buf
}
