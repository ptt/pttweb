package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-redis/redis"
	"github.com/ptt/pttweb/page"
	"github.com/rvelhote/go-recaptcha"
)

const (
	RecaptchaURL       = `https://www.google.com/recaptcha/api/siteverify`
	GRecaptchaResponse = `g-recaptcha-response`
	CaptchaHandle      = `handle`

	MaxCaptchaHandleLength = 80
)

var (
	ErrCaptchaHandleNotFound = &CaptchaErr{
		error:          errors.New("captcha handle not found"),
		SetCaptchaPage: func(p *page.Captcha) { p.CaptchaErr.IsNotFound = true },
	}
	ErrCaptchaVerifyFailed = &CaptchaErr{
		error:          errors.New("captcha verfication failed"),
		SetCaptchaPage: func(p *page.Captcha) { p.CaptchaErr.IsVerifyFailed = true },
	}
)

var captchaRedis *redis.Client

type CaptchaErr struct {
	error
	SetCaptchaPage func(p *page.Captcha)
}

func initCaptchaRedisServer(c *CaptchaRedisConfig) error {
	captchaRedis = redis.NewClient(&redis.Options{
		Network:  c.Network,
		Addr:     c.Addr,
		Password: c.Password,
		DB:       c.DB,
	})
	return nil
}

func handleCaptcha(c *Context, w http.ResponseWriter) error {
	p, err := handleCaptchaInternal(c, w)
	if err != nil {
		return err
	}
	return page.ExecutePage(w, p)
}

func handleCaptchaInternal(c *Context, w http.ResponseWriter) (*page.Captcha, error) {
	p := &page.Captcha{
		Handle:           c.R.FormValue(CaptchaHandle),
		RecaptchaSiteKey: config.RecaptchaSiteKey,
	}
	if u, err := router.Get("captcha").URLPath(); err != nil {
		return nil, err
	} else {
		q := make(url.Values)
		q.Set(CaptchaHandle, c.R.FormValue(CaptchaHandle))
		p.PostAction = u.String() + "?" + q.Encode()
	}
	// Check if the handle is valid.
	if _, err := fetchVerificationKey(p.Handle); err != nil {
		return translateCaptchaErr(p, err)
	}
	if response := c.R.PostFormValue(GRecaptchaResponse); response != "" {
		r := recaptcha.Recaptcha{
			PrivateKey: config.RecaptchaSecret,
			URL:        RecaptchaURL,
		}
		verifyResp, errs := r.Verify(response, "")
		if len(errs) != 0 || !verifyResp.Success {
			p.InternalErrMessage = fmt.Sprintf("%v", errs)
			return translateCaptchaErr(p, ErrCaptchaVerifyFailed)
		} else if verifyResp.Success {
			var err error
			p.VerificationKey, err = fetchVerificationKey(p.Handle)
			if err != nil {
				return translateCaptchaErr(p, err)
			}
		}
	}
	return p, nil
}

func translateCaptchaErr(p *page.Captcha, err error) (*page.Captcha, error) {
	if ce, ok := err.(*CaptchaErr); ok && ce.SetCaptchaPage != nil {
		ce.SetCaptchaPage(p)
		if p.InternalErrMessage == "" {
			p.InternalErrMessage = fmt.Sprintf("%v", err)
		}
		return p, nil
	}
	return p, err
}

func fetchVerificationKey(handle string) (string, error) {
	if len(handle) > MaxCaptchaHandleLength {
		return "", ErrCaptchaHandleNotFound
	}
	data, err := captchaRedis.Get(handle).Result()
	if err == redis.Nil {
		return "", ErrCaptchaHandleNotFound
	} else if err != nil {
		return "", err
	}
	e, err := decodeCaptchaEntry(data)
	if err != nil {
		return "", err
	}
	return e.Verify, nil
}

func handleCaptchaInsert(c *Context, w http.ResponseWriter) error {
	secret := c.R.FormValue("secret")
	handle := c.R.FormValue("handle")
	verify := c.R.FormValue("verify")
	if secret == "" || handle == "" || verify == "" || len(handle) > MaxCaptchaHandleLength {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	if secret != config.CaptchaInsertSecret {
		w.WriteHeader(http.StatusForbidden)
		return nil
	}
	data, err := encodeCaptchaEntry(&CaptchaEntry{
		Handle: handle,
		Verify: verify,
	})
	if err != nil {
		return err
	}
	expire := time.Duration(config.CaptchaExpireSecs) * time.Second
	r, err := captchaRedis.SetNX(handle, data, expire).Result()
	if err != nil {
		return err
	}
	if !r {
		w.WriteHeader(http.StatusConflict)
		return nil
	}
	return nil
}

type CaptchaEntry struct {
	Handle string `json:"h"`
	Verify string `json:"v"`
}

func encodeCaptchaEntry(e *CaptchaEntry) (string, error) {
	buf, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func decodeCaptchaEntry(data string) (*CaptchaEntry, error) {
	var e CaptchaEntry
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		return nil, err
	}
	return &e, nil
}
