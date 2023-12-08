package telebot

import (
	"github.com/graphomania/tg/scheduler"
	"github.com/stretchr/testify/require"
	"log"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestConservative(t *testing.T) {
	token := os.Getenv("TG_TEST_TOKEN")
	chat, err := strconv.ParseInt(os.Getenv("TG_TEST_CHAT"), 10, 64)
	require.NoError(t, err)

	alb := Album{}
	for i := 0; i < 10; i++ {
		alb = append(alb, &Photo{File: FromDisk("testdata/pic.jpg")})
	}

	bot, err := NewBot(Settings{
		Token:  token,
		Poller: &LongPoller{Timeout: time.Minute},
		//Verbose: true,
		Synchronous: true,
		OnError: func(err error, context Context) {
			require.NoError(t, err)
		},
		Scheduler: scheduler.Default(),
	})
	require.NoError(t, err)

	for i := 0; i < 1; i++ {
		log.Printf("%d.\n", i)
		_, err := bot.SendAlbum(&Chat{ID: chat}, alb[0:(10-i)])
		require.NoError(t, err)
	}

	for i := 0; i < 30; i++ {
		_, err := bot.Send(&Chat{ID: chat}, strconv.Itoa(i))
		require.NoError(t, err)
	}
}
