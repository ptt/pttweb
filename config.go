package main

import (
	"errors"

	"github.com/ptt/pttweb/captcha"
	"github.com/ptt/pttweb/experiment"
	"github.com/ptt/pttweb/extcache"
)

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

	// EnableLinkOriginalInAllPost indicates whether to parse the board name in
	// ALLPOST board and link to original posts.
	EnableLinkOriginalInAllPost bool

	FeedPrefix            string
	AtomFeedTitleTemplate string

	EnablePushStream            bool
	PushStreamSharedSecret      string
	PushStreamSubscribeLocation string

	RecaptchaSiteKey    string
	RecaptchaSecret     string
	CaptchaInsertSecret string
	CaptchaExpireSecs   int
	CaptchaRedisConfig  *captcha.RedisConfig

	ExtCacheConfig extcache.Config

	Experiments Experiments
}

type Experiments struct {
	ExtCache experiment.OptIn
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

func (c *PttwebConfig) captchaConfig() *captcha.Config {
	enabled := c.RecaptchaSiteKey != "" && c.RecaptchaSecret != "" && c.CaptchaRedisConfig != nil
	return &captcha.Config{
		Enabled:      enabled,
		InsertSecret: c.CaptchaInsertSecret,
		ExpireSecs:   c.CaptchaExpireSecs,
		Recaptcha: captcha.RecaptchaConfig{
			SiteKey: c.RecaptchaSiteKey,
			Secret:  c.RecaptchaSecret,
		},
		Redis: *c.CaptchaRedisConfig,
	}
}
