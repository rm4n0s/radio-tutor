package admin

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// deleteStudent deletes the profile of the student, then their progress, their
// photos and their hidden-cassette entry (SPEC §3.1). The profile goes first, so
// that a refusal (the last admin) leaves everything else alone. Tags:
// AdminDeleteCancelled, AdminPhotosUsernameInvalid, AdminPhotosDeleteFailed and
// the tags of profile.Store.Delete, progress.Store.DeleteUser and
// settings.Store.Update.
func deleteStudent(ctx context.Context, env *app.Env, username string) error {
	if err := ctx.Err(); err != nil {
		return errors.NewErr("AdminDeleteCancelled", err).SetMetadata("username", username)
	}
	if err := env.Profiles.Delete(ctx, username); err != nil {
		return err
	}
	if err := env.Progress.DeleteUser(ctx, username); err != nil {
		return err
	}
	if err := removePhotosOf(env, username); err != nil {
		return err
	}
	return env.Settings.Update(ctx, func(s *settings.Settings) { delete(s.HiddenCassettes, username) })
}

// removePhotosOf deletes photos/<username>. A student without photos is not an
// error.
func removePhotosOf(env *app.Env, username string) error {
	if !safeElement(username) {
		return errors.New("AdminPhotosUsernameInvalid", "the username cannot name a photos folder", "username", username)
	}
	dir := filepath.Join(env.Paths.PhotosDir(), username)
	if err := os.RemoveAll(dir); err != nil {
		return errors.NewErr("AdminPhotosDeleteFailed", err).SetMetadata("path", dir)
	}
	return nil
}

// removeCassetteData deletes the progress and the photos of every student for
// the cassette (the checkbox of SPEC §6.2): the save files, and
// photos/<username>/<id> for every username. Tags: AdminPhotosCassetteInvalid,
// AdminPhotosListFailed, AdminPhotosDeleteFailed and the tags of
// progress.Store.DeleteCassette.
func removeCassetteData(ctx context.Context, env *app.Env, id string) error {
	if !cassette.ValidCassetteID(id) {
		return errors.New("AdminPhotosCassetteInvalid", "not a cassette id", "id", id)
	}
	if err := env.Progress.DeleteCassette(ctx, id); err != nil {
		return err
	}
	root := env.Paths.PhotosDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return errors.NewErr("AdminPhotosListFailed", err).SetMetadata("path", root)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name(), id)
		if err := os.RemoveAll(dir); err != nil {
			return errors.NewErr("AdminPhotosDeleteFailed", err).SetMetadata("path", dir)
		}
	}
	return nil
}

// safeElement reports whether name is a single, plain path element.
func safeElement(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name
}
