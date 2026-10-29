package submission

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/storage"
)

// photoDir returns the folder of a submission's photos (SPEC §4):
// photos/<username>/<cassette-id>/<submission-id>. The three names were
// checked by the progress store before a session exists.
func photoDir(photosDir, username, cassetteID, submissionID string) string {
	return filepath.Join(photosDir, username, cassetteID, submissionID)
}

// photoName is the file name of the n-th photo (n from 1).
func photoName(n int) string { return strconv.Itoa(n) + ".jpg" }

// storePhotos writes the photos as <n>.jpg into dir (made with mode 0700, files
// 0600) and returns the file names. The photos are the normalized JPEGs the
// phone server passes; they are stored as they are. A failure removes what was
// written.
func storePhotos(dir string, photos [][]byte) ([]string, error) {
	if err := storage.EnsureDir(dir); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(photos))
	for i, data := range photos {
		name := photoName(i + 1)
		if err := writePhoto(filepath.Join(dir, name), data); err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// writePhoto creates the file with mode 0600, writes data and syncs it. It
// fails with the tags SubmissionPhotoCreateFailed and SubmissionPhotoWriteFailed.
func writePhoto(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, storage.FileMode)
	if err != nil {
		return errors.NewErr("SubmissionPhotoCreateFailed", err).SetMetadata("path", path)
	}
	return writeAndClose(f, path, data)
}

// writeAndClose writes data to the open file, syncs and closes it.
func writeAndClose(f *os.File, path string, data []byte) error {
	_, err := f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return errors.NewErr("SubmissionPhotoWriteFailed", err).SetMetadata("path", path)
	}
	return nil
}

// loadPhotos reads the stored photos of a submission, in order.
func loadPhotos(dir string, names []string) ([][]byte, error) {
	photos := make([][]byte, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, errors.NewErr("SubmissionPhotoReadFailed", err).SetMetadata("path", filepath.Join(dir, name))
		}
		photos = append(photos, data)
	}
	return photos, nil
}
