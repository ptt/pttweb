package main

import "errors"

type PttwebConfig struct {
	Bind              []string
	BoarddAddress     string
	SearchAddress     string
	MandAddress       string
	MemcachedAddress  string
	TemplateDirectory string
	StaticPrefix      string
	SitePrefix        string

	MemcachedMaxConn int

	GAAccount string
	GADomain  string

	EnableOver18Cookie bool

	FeedPrefix            string
	AtomFeedTitleTemplate string

	EnablePushStream            bool
	PushStreamSharedSecret      string
	PushStreamSubscribeLocation string

	RecaptchaSiteKey    string
	RecaptchaSecret     string
	CaptchaInsertSecret string
	CaptchaExpireSecs   int
	CaptchaRedisConfig  *CaptchaRedisConfig
}

// See https://godoc.org/github.com/go-redis/redis#Options
type CaptchaRedisConfig struct {
	Network  string
	Addr     string
	Password string
	DB       int
}

const (
	DefaultBoarddMaxConn    = 16
	DefaultMemcachedMaxConn = 16
)

func (c *PttwebConfig) CheckAndFillDefaults() error {
	if c.BoarddAddress == "" {
		return errors.New("boardd address not specified")
	}

	if c.MemcachedAddress == "" {
		return errors.New("memcached address not specified")
	}

	if c.MemcachedMaxConn <= 0 {
		c.MemcachedMaxConn = DefaultMemcachedMaxConn
	}

	return nil
}

func (c *PttwebConfig) IsCaptchaConfigured() bool {
	return c.RecaptchaSiteKey != "" && c.RecaptchaSecret != "" && c.CaptchaRedisConfig != nil
}
