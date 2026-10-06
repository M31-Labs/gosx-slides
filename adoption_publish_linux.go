//go:build linux

package slides

import "golang.org/x/sys/unix"

func publishAdoptionDirectory(stage, destination string) error {
	// RENAME_NOREPLACE closes the check/rename race, including an empty directory
	// created by someone else while this migration was being prepared.
	return unix.Renameat2(unix.AT_FDCWD, stage, unix.AT_FDCWD, destination, unix.RENAME_NOREPLACE)
}
