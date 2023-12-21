package telebot

import (
	"fmt"
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
		alb = append(alb, &Photo{File: FromDisk("testdata/pic.jpg"), Caption: fmt.Sprint(i + 1)})
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
		Retries:   1,
	})
	require.NoError(t, err)

	for i := 0; i < 400; i++ {
		log.Printf("%d.\n", i)
		//_, err := bot.SendAlbum(&Chat{ID: chat}, alb[0:(((i+1)*2)%9+1)])
		_, err := bot.SendAlbum(&Chat{ID: chat}, alb[0:10])
		require.NoError(t, err)
	}
}
