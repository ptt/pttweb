package extcache

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"hash/fnv"
	"net/url"
	"strconv"
	"time"
)

type ctxExtCacheKey struct{}

var ctxKey = (*ctxExtCacheKey)(nil)

func WithExtCache(ctx context.Context, cache ExtCache) context.Context {
	if cache == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey, cache)
}

func FromContext(ctx context.Context) (ExtCache, bool) {
	c, ok := ctx.Value(ctxKey).(ExtCache)
	return c, ok
}

type ExtCache interface {
	Generate(urlStr string) (string, error)
}

type Config struct {
	Enabled    bool
	Prefix     string
	HashPrefix string
	Secret     string
	Expires    int
}

type extCache struct {
	cfg Config
}

func New(cfg Config) ExtCache {
	if !cfg.Enabled {
		return nil
	}
	return &extCache{cfg: cfg}
}

func (c *extCache) Generate(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	uri := "/" + u.Scheme + "/" + u.Host + u.Path
	expire := snapExpire(uri, time.Now().Unix(), int64(c.cfg.Expires))
	expireStr := strconv.FormatInt(expire, 10)
	h := md5.Sum([]byte(expireStr + c.cfg.HashPrefix + uri + c.cfg.Secret))
	sig := base64.RawURLEncoding.EncodeToString(h[:])

	q := make(url.Values)
	q.Set("e", expireStr)
	q.Set("s", sig)

	return c.cfg.Prefix + uri + "?" + q.Encode(), nil
}

func snapExpire(key string, now, min int64) int64 {
	h := fnv.New32()
	_, _ = h.Write([]byte(key))
	expire := (now+min+0xFFFF)&^0xFFFF + int64(h.Sum32()&0xFFFF)
	return expire
}
