//go:build darwin

package slides

import "golang.org/x/sys/unix"

func publishAdoptionDirectory(stage, destination string) error {
	return unix.RenamexNp(stage, destination, unix.RENAME_EXCL)
}
