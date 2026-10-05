//go:build !linux && !darwin

package slides

import "os"

func publishAdoptionDirectory(stage, destination string) error {
	// Windows refuses an existing destination directory. On other systems the
	// final existence check is in adoptionPublish; authored nonempty directories
	// cannot be replaced by this directory rename.
	return os.Rename(stage, destination)
}
