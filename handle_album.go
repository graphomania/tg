package telebot

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type AlbumHandlerFunc func(cs []Context) error

func (f AlbumHandlerFunc) ToHandlerFunc() HandlerFunc {
	return func(c Context) error {
		return f([]Context{c})
	}
}

// HandleAlbum opts -- MiddlewareFunc / endpoints (OnPhoto, OnVideo...) -- default=OnMedia.
// At the time being, I have no time for global Telebot refactoring,
// so Album handling overrides singe media handling.
func (b *Bot) HandleAlbum(handler AlbumHandlerFunc, opts ...interface{}) {
	b.Group().HandleAlbum(handler, opts...)
}

func (g *Group) HandleAlbum(handler AlbumHandlerFunc, opts ...interface{}) {
	albumHandler := albumHandleManager{
		Group:         g,
		Handler:       handler,
		Timeout:       time.Second / 4,
		albums:        map[string][]Context{},
		registerMutex: sync.Mutex{},
	}

	endpoints := make([]interface{}, 0)
	middlewares := make([]MiddlewareFunc, 0)
	for _, opt := range opts {
		switch o := opt.(type) {
		case MiddlewareFunc:
			middlewares = append(middlewares, o)
		default:
			endpoints = append(endpoints, o)
		}
	}
	if len(endpoints) == 0 {
		endpoints = append(endpoints, OnMedia)
	}

	for _, endpoint := range endpoints {
		albumHandler.Group.Handle(endpoint, func(ctx Context) error { return albumHandler.register(ctx) }, middlewares...)
	}
}

type albumHandleManager struct {
	Group   *Group
	Handler AlbumHandlerFunc
	Timeout time.Duration

	albums        map[string][]Context
	registerMutex sync.Mutex
}

func (handler *albumHandleManager) register(ctx Context) error {
	defer handler.registerMutex.Unlock()
	handler.registerMutex.Lock()

	id := mediaGroupToId(ctx.Message())
	if _, contains := handler.albums[id]; !contains {
		handler.albums[id] = []Context{ctx}

		go handler.delayHandling(ctx, id)
	} else {
		handler.albums[id] = append(handler.albums[id], ctx)
	}

	return nil
}

func (handler *albumHandleManager) delayHandling(ctx Context, id string) {
	message := ctx.Message()
	defer func() {
		delete(handler.albums, id)
		if r := recover(); r != nil {
			ctx.Bot().OnError(errors.New(fmt.Sprintf("%v", r)), ctx)
		}
	}()
	if message.AlbumID != "" { // no need to delay handling of single medias
		time.Sleep(handler.Timeout)
	}
	contexts := handler.albums[mediaGroupToId(message)]
	sort.Slice(contexts, func(i, j int) bool {
		return contexts[i].Message().ID < contexts[j].Message().ID
	})
	err := handler.Handler(contexts)
	if err != nil {
		ctx.Bot().OnError(err, ctx)
	}
}

func mediaGroupToId(msg *Message) string {
	if msg.AlbumID != "" {
		return msg.AlbumID
	} else {
		return fmt.Sprintf("%d_%d", msg.Chat.ID, msg.ID)
	}
}
