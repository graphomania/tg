package telebot

import (
	"io"
	"os"
)

// Local submodule handles weird tg style of handling local servers.
// TODO: Requires more research into the task. WORK IN PROGRESS.
type Local interface {
	Download(b *Bot, file *File, dst string) error
}

var _ Local = LocalCopy{}
var _ Local = LocalMove{}

// LocalCopy copies the file from local telegram-bot-api data directory to dst,
// providing the path to original copy to file.FileLocal.
type LocalCopy struct{}

func (loc LocalCopy) Download(b *Bot, file *File, dst string) error {
	localPath := file.FilePath
	if file.FilePath == "" {
		f, err := b.FileByID(file.FileID)
		if err != nil {
			return err
		}
		// FilePath is updated, allowing user to delete the file from the local server's cache
		localPath = f.FilePath
		file.FilePath = localPath
	}

	reader, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	out, err := os.Create(dst)
	if err != nil {
		return wrapError(err)
	}
	defer out.Close()

	if _, err := io.Copy(out, reader); err != nil {
		return wrapError(err)
	}

	file.FileLocal = localPath
	return nil
}

// LocalMove move the file from telegram-bot-api directory, re-uploading are not promised,
// if you care about possible multiple file downloads, you should consider LocalCopy.
type LocalMove struct{}

func (loc LocalMove) Download(b *Bot, file *File, dst string) error {
	localPath := file.FilePath
	if file.FilePath == "" {
		f, err := b.FileByID(file.FileID)
		if err != nil {
			return err
		}
		localPath = f.FilePath
		file.FilePath = localPath
	}

	if err := os.Rename(localPath, dst); err != nil {
		return wrapError(err)
	}

	file.FileLocal = dst
	return nil
}
