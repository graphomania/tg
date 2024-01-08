package videoutil

import (
	"github.com/graphomania/tg"
	"github.com/graphomania/tg/photoutil"
	"github.com/graphomania/tg/scheduler"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

func prelude() error {
	if err := os.Chdir(".."); err != nil {
		return err
	}

	if _, err := os.Stat("testdata"); err != nil {
		if err := os.Mkdir("testdata", 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat("testdata/pic.jpg"); err != nil {
		if _, err := exec.Command("wget", "-O", "testdata/pic.jpg", "https://github.com/graphomania/testdata/raw/main/pic.jpg").Output(); err != nil {
			return err
		}
	}
	if _, err := os.Stat("testdata/vid.mp4"); err != nil {
		if _, err := exec.Command("wget", "-O", "testdata/vid.mp4", "https://github.com/graphomania/testdata/raw/main/gio.mp4").Output(); err != nil {
			return err
		}
	}

	return nil
}

func TestUtilities(t *testing.T) {
	require.NoError(t, prelude())

	testPhoto := "testdata/pic.jpg"
	testVideo := "testdata/vid.mp4"
	testVideoM4V := "testdata/vid.m4v"
	options := &Opt{Preset: "fast", TmpDir: "testdata"}
	token := os.Getenv("TG_TEST_TOKEN")
	chatID, err := strconv.ParseInt(os.Getenv("TG_TEST_CHAT"), 10, 64)
	require.NoError(t, err)
	chat := &telebot.Chat{ID: chatID}

	bot, err := telebot.NewBot(telebot.Settings{
		Token:     token,
		Local:     telebot.LocalMoving(),
		OnError:   telebot.OnErrorLog(),
		Verbose:   true,
		Scheduler: scheduler.Default(),
	})
	require.NoError(t, err)

	require.NoError(t, err)

	vid := telebot.Video{File: telebot.FromDisk(testVideoM4V)}.With(Timed(ExtraFormats(options), ThumbnailAt(0.5, options)))
	_, err = bot.SendAlbum(chat, telebot.Album{vid, vid})
	require.NoError(t, err)

	_, err = bot.Send(chat, telebot.Photo{File: telebot.FromDisk(testPhoto)}.
		With(photoutil.Converted(&photoutil.Opt{Width: 600, Height: 600})))
	require.NoError(t, err)

	_, err = bot.Send(chat, telebot.Video{File: telebot.FromDisk(testVideo), NoStreaming: true}.
		With(Timed(ThumbnailAt(0.5, options))))
	require.NoError(t, err)

	_, err = bot.Send(chat, telebot.Video{File: telebot.FromDisk(testVideo), NoStreaming: true}.
		With(Timed(ThumbnailFrom(testPhoto, options))))
	require.NoError(t, err)

	_, err = bot.Send(chat, telebot.Video{File: telebot.FromDisk(testVideo), NoStreaming: true}.
		With(Timed(Converted(options), ThumbnailFrom(testPhoto, options), WithMetadata())))
	require.NoError(t, err)

	_, err = bot.Send(chat, telebot.Video{File: telebot.FromDisk(testVideo)}.
		With(Timed(Converted(options), ThumbnailAt(0.5), Muted())))

	require.NoError(t, err)
}
