package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// createDirLink creates a directory link at `link` pointing to `target`.
// On Unix it uses a symbolic link; on Windows it uses a directory junction
// (mklink /J) so an unprivileged user can create it. Falls back to
// os.Symlink on Windows when mklink fails.
func createDirLink(target, link string) error {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return os.Symlink(absTarget, link)
	}
	absLink, err := filepath.Abs(link)
	if err != nil {
		return err
	}
	out, mkErr := exec.Command("cmd", "/c", "mklink", "/J", absLink, absTarget).CombinedOutput()
	if mkErr == nil {
		return nil
	}
	if symErr := os.Symlink(absTarget, link); symErr == nil {
		return nil
	}
	return fmt.Errorf("mklink /J failed: %s: %w", string(out), mkErr)
}

// createFileLink creates a link to a file. On Unix this is a symlink; on
// Windows it tries `mklink /H` (a hard link, which needs no admin rights on
// NTFS) and falls back to os.Symlink, then to copying the file as a last
// resort so non-Claude platforms always end up with a readable file.
func createFileLink(target, link string) error {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return os.Symlink(absTarget, link)
	}
	absLink, err := filepath.Abs(link)
	if err != nil {
		return err
	}
	if out, mkErr := exec.Command("cmd", "/c", "mklink", "/H", absLink, absTarget).CombinedOutput(); mkErr == nil {
		return nil
	} else if symErr := os.Symlink(absTarget, link); symErr == nil {
		return nil
	} else {
		// Last resort: copy the file so the install still produces a usable
		// artefact when neither hard links nor symlinks are available.
		if copyErr := copyFile(absTarget, absLink); copyErr == nil {
			return nil
		}
		return fmt.Errorf("mklink /H failed: %s: %w", string(out), mkErr)
	}
}

// replaceLink removes any existing link at `link` and creates a fresh link
// of the requested kind (file or directory). A non-link entry at the path
// is left alone and an error is returned so we never destroy real files.
func replaceLink(target, link string, isDir bool) error {
	if info, err := os.Lstat(link); err == nil {
		if info.Mode()&os.ModeSymlink == 0 && !isJunction(link) {
			return fmt.Errorf("%s exists and is not a symlink; refusing to overwrite", link)
		}
		if rmErr := os.Remove(link); rmErr != nil {
			return fmt.Errorf("remove existing link %s: %w", link, rmErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", link, err)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(link), err)
	}
	if isDir {
		return createDirLink(target, link)
	}
	return createFileLink(target, link)
}

// isJunction returns true on Windows when the path is a directory junction
// (which Lstat does not flag as a symlink). On Unix it is always false.
func isJunction(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	// Directory junctions appear as directories with the reparse-point
	// attribute set; checking IsDir + ModeIrregular is the closest portable
	// hint without pulling in golang.org/x/sys/windows.
	return info.IsDir() && info.Mode()&os.ModeIrregular != 0
}
