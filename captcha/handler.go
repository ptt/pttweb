package captcha

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
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

type CaptchaErr struct {
	error
	SetCaptchaPage func(p *page.Captcha)
}

type Handler struct {
	config      *Config
	router      *mux.Router
	redisClient *redis.Client
}

func Install(cfg *Config, r *mux.Router) error {
	redisClient := redis.NewClient(&redis.Options{
		Network:  cfg.Redis.Network,
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	h := &Handler{
		config:      cfg,
		router:      r,
		redisClient: redisClient,
	}
	h.installRoutes(r)
	return nil
}

func (h *Handler) installRoutes(r *mux.Router) {
	r.Path(`/captcha`).
		Handler(page.ErrorWrapper(h.handleCaptcha)).
		Name("captcha")

	r.Path(`/captcha/insert`).
		Handler(page.ErrorWrapper(h.handleCaptchaInsert)).
		Name("captcha_insert")
}

func (h *Handler) handleCaptcha(ctx page.Context, w http.ResponseWriter) error {
	p, err := h.handleCaptchaInternal(ctx, w)
	if err != nil {
		return err
	}
	return page.ExecutePage(w, p)
}

func (h *Handler) handleCaptchaInternal(ctx page.Context, w http.ResponseWriter) (*page.Captcha, error) {
	p := &page.Captcha{
		Handle:           ctx.Request().FormValue(CaptchaHandle),
		RecaptchaSiteKey: h.config.Recaptcha.SiteKey,
	}
	if u, err := h.router.Get("captcha").URLPath(); err != nil {
		return nil, err
	} else {
		q := make(url.Values)
		q.Set(CaptchaHandle, ctx.Request().FormValue(CaptchaHandle))
		p.PostAction = u.String() + "?" + q.Encode()
	}
	// Check if the handle is valid.
	if _, err := h.fetchVerificationKey(p.Handle); err != nil {
		return translateCaptchaErr(p, err)
	}
	if response := ctx.Request().PostFormValue(GRecaptchaResponse); response != "" {
		r := recaptcha.Recaptcha{
			PrivateKey: h.config.Recaptcha.Secret,
			URL:        RecaptchaURL,
		}
		verifyResp, errs := r.Verify(response, "")
		if len(errs) != 0 || !verifyResp.Success {
			p.InternalErrMessage = fmt.Sprintf("%v", errs)
			return translateCaptchaErr(p, ErrCaptchaVerifyFailed)
		} else if verifyResp.Success {
			var err error
			p.VerificationKey, err = h.fetchVerificationKey(p.Handle)
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

func (h *Handler) fetchVerificationKey(handle string) (string, error) {
	if len(handle) > MaxCaptchaHandleLength {
		return "", ErrCaptchaHandleNotFound
	}
	data, err := h.redisClient.Get(handle).Result()
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

func (h *Handler) handleCaptchaInsert(ctx page.Context, w http.ResponseWriter) error {
	req := ctx.Request()
	secret := req.FormValue("secret")
	handle := req.FormValue("handle")
	verify := req.FormValue("verify")
	if secret == "" || handle == "" || verify == "" || len(handle) > MaxCaptchaHandleLength {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	if secret != h.config.InsertSecret {
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
	expire := time.Duration(h.config.ExpireSecs) * time.Second
	r, err := h.redisClient.SetNX(handle, data, expire).Result()
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
