package captcha

type Config struct {
	Enabled      bool
	InsertSecret string
	ExpireSecs   int
	Recaptcha    RecaptchaConfig
	Redis        RedisConfig
}

type RecaptchaConfig struct {
	SiteKey string
	Secret  string
}

// See https://godoc.org/github.com/go-redis/redis#Options
type RedisConfig struct {
	Network  string
	Addr     string
	Password string
	DB       int
}
