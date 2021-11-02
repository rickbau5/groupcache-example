package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mailgun/groupcache"
)

type Data struct {
	GUID        string    `json:"guid"`
	DateCreated time.Time `json:"date_created"`
}

type Backend interface {
	Get(ctx context.Context, guid string) (*Data, error)
}

type backendCacheImpl struct {
	cache groupcache.Getter
}

func (b backendCacheImpl) Get(ctx context.Context, guid string) (*Data, error) {
	var value groupcache.ByteView

	// load from cache
	err := b.cache.Get(ctx, guid, groupcache.ByteViewSink(&value))
	if err != nil {
		return nil, fmt.Errorf("failed getting from cache: %w", err)
	}

	// unmarshal bytes - in the future this could be a groupcache#ProtoSink
	var data Data
	if err := json.NewDecoder(value.Reader()).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed unmarshaling data from cache: %w", err)
	}

	// return data
	return &data, nil
}

type backendImpl struct{}

func (b backendImpl) Get(ctx context.Context, guid string) (*Data, error) {
	return &Data{
		GUID:        guid,
		DateCreated: time.Now(),
	}, nil
}
