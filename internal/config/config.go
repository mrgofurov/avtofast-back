package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App struct {
		Name        string `yaml:"name"`
		Environment string `yaml:"environment"`
		Port        int    `yaml:"port"`
		Prefork     bool   `yaml:"prefork"`
		APIBaseURL  string `yaml:"api_base_url"`
	} `yaml:"app"`

	Postgres struct {
		Host            string        `yaml:"host"`
		Port            int           `yaml:"port"`
		User            string        `yaml:"user"`
		Password        string        `yaml:"password"`
		Database        string        `yaml:"database"`
		SSLMode         string        `yaml:"ssl_mode"`
		MaxConns        int32         `yaml:"max_conns"`
		MinConns        int32         `yaml:"min_conns"`
		MaxConnLifetime time.Duration `yaml:"max_conn_lifetime"`
	} `yaml:"postgres"`

	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
		PoolSize int    `yaml:"pool_size"`
	} `yaml:"redis"`

	JWT struct {
		SecretKey string `yaml:"secret_key"`
		Issuer    string `yaml:"issuer"`
		Audience  string `yaml:"audience"`
		// AccessTTL is short enough that a revoked or role-changed session
		// stops working within the day; RefreshTTL is what keeps a learner
		// signed in on a phone they only open twice a week.
		AccessTTL  time.Duration `yaml:"access_ttl"`
		RefreshTTL time.Duration `yaml:"refresh_ttl"`
	} `yaml:"jwt"`

	Firebase struct {
		// ProjectID enables verification of the Firebase ID tokens the mobile
		// app gets from Google/Apple sign-in. Empty disables the exchange
		// endpoint rather than accepting unverified tokens.
		ProjectID string `yaml:"project_id"`
	} `yaml:"firebase"`

	Signing struct {
		Ed25519PrivateKeyBase64 string `yaml:"ed25519_private_key"`
		Ed25519PublicKeyBase64  string `yaml:"ed25519_public_key"`
	} `yaml:"signing"`

	Features struct {
		Friends           bool `yaml:"friends"`
		WeeklyChallenges  bool `yaml:"weekly_challenges"`
		VoiceExplanations bool `yaml:"voice_explanations"`
		AIMistakeAnalysis bool `yaml:"ai_mistake_analysis"`
	} `yaml:"features"`

	Rules struct {
		ExamQuestionCount int `yaml:"exam_question_count"`
		ExamDurationSec   int `yaml:"exam_duration_sec"`
		ExamPassCount     int `yaml:"exam_pass_count"`
	} `yaml:"rules"`

	// Parsed runtime keys
	Ed25519PrivKey ed25519.PrivateKey `yaml:"-"`
	Ed25519PubKey  ed25519.PublicKey  `yaml:"-"`
}

func Load(path string) (*Config, error) {
	// Automatically load .env file if available
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	cfg := &Config{}

	// Defaults
	cfg.App.Name = "avtofast-api"
	cfg.App.Environment = "development"
	cfg.App.Port = 8080
	cfg.App.Prefork = false
	cfg.App.APIBaseURL = "https://api.avtofast.uz/v1"

	cfg.Postgres.Host = "localhost"
	cfg.Postgres.Port = 5432
	cfg.Postgres.User = "postgres"
	cfg.Postgres.Password = "postgres"
	cfg.Postgres.Database = "avtofast"
	cfg.Postgres.SSLMode = "disable"
	cfg.Postgres.MaxConns = 80
	cfg.Postgres.MinConns = 20
	cfg.Postgres.MaxConnLifetime = 30 * time.Minute

	cfg.Redis.Addr = "localhost:6379"
	cfg.Redis.PoolSize = 100

	cfg.JWT.SecretKey = "avtofast-super-secure-dev-secret-key-32b"
	cfg.JWT.Issuer = "https://api.avtofast.uz/auth"
	cfg.JWT.Audience = "avtofast-api"
	cfg.JWT.AccessTTL = 24 * time.Hour
	cfg.JWT.RefreshTTL = 90 * 24 * time.Hour
	cfg.Firebase.ProjectID = "avtofast-8fba6"

	cfg.Rules.ExamQuestionCount = 20
	cfg.Rules.ExamDurationSec = 1200
	cfg.Rules.ExamPassCount = 16

	// Load from yaml config if provided
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			_ = yaml.Unmarshal(data, cfg)
		}
	}

	// Environment variable overrides (.env or OS environment)
	if name := os.Getenv("APP_NAME"); name != "" {
		cfg.App.Name = name
	}
	if env := os.Getenv("APP_ENV"); env != "" {
		cfg.App.Environment = env
	}
	if p := os.Getenv("PORT"); p != "" {
		if val, err := strconv.Atoi(p); err == nil {
			cfg.App.Port = val
		}
	}
	if prefork := os.Getenv("PREFORK"); prefork != "" {
		cfg.App.Prefork = (prefork == "true" || prefork == "1")
	}
	if apiBase := os.Getenv("API_BASE_URL"); apiBase != "" {
		cfg.App.APIBaseURL = apiBase
	}

	// Postgres envs
	if h := os.Getenv("POSTGRES_HOST"); h != "" {
		cfg.Postgres.Host = h
	}
	if pgPort := os.Getenv("POSTGRES_PORT"); pgPort != "" {
		if val, err := strconv.Atoi(pgPort); err == nil {
			cfg.Postgres.Port = val
		}
	}
	if u := os.Getenv("POSTGRES_USER"); u != "" {
		cfg.Postgres.User = u
	}
	if pw := os.Getenv("POSTGRES_PASSWORD"); pw != "" {
		cfg.Postgres.Password = pw
	}
	if db := os.Getenv("POSTGRES_DB"); db != "" {
		cfg.Postgres.Database = db
	}
	if ssl := os.Getenv("POSTGRES_SSL_MODE"); ssl != "" {
		cfg.Postgres.SSLMode = ssl
	}
	if maxC := os.Getenv("POSTGRES_MAX_CONNS"); maxC != "" {
		if val, err := strconv.Atoi(maxC); err == nil {
			cfg.Postgres.MaxConns = int32(val)
		}
	}
	if minC := os.Getenv("POSTGRES_MIN_CONNS"); minC != "" {
		if val, err := strconv.Atoi(minC); err == nil {
			cfg.Postgres.MinConns = int32(val)
		}
	}

	// Redis envs
	if rAddr := os.Getenv("REDIS_ADDR"); rAddr != "" {
		cfg.Redis.Addr = rAddr
	}
	if rPass := os.Getenv("REDIS_PASSWORD"); rPass != "" {
		cfg.Redis.Password = rPass
	}
	if rDB := os.Getenv("REDIS_DB"); rDB != "" {
		if val, err := strconv.Atoi(rDB); err == nil {
			cfg.Redis.DB = val
		}
	}
	if rPool := os.Getenv("REDIS_POOL_SIZE"); rPool != "" {
		if val, err := strconv.Atoi(rPool); err == nil {
			cfg.Redis.PoolSize = val
		}
	}

	// JWT envs
	if jwtSec := os.Getenv("JWT_SECRET"); jwtSec != "" {
		cfg.JWT.SecretKey = jwtSec
	}
	if jwtIss := os.Getenv("JWT_ISSUER"); jwtIss != "" {
		cfg.JWT.Issuer = jwtIss
	}
	if jwtAud := os.Getenv("JWT_AUDIENCE"); jwtAud != "" {
		cfg.JWT.Audience = jwtAud
	}
	if ttl := os.Getenv("JWT_ACCESS_TTL"); ttl != "" {
		if val, err := time.ParseDuration(ttl); err == nil {
			cfg.JWT.AccessTTL = val
		}
	}
	if ttl := os.Getenv("JWT_REFRESH_TTL"); ttl != "" {
		if val, err := time.ParseDuration(ttl); err == nil {
			cfg.JWT.RefreshTTL = val
		}
	}

	// Firebase envs
	if projectID := os.Getenv("FIREBASE_PROJECT_ID"); projectID != "" {
		cfg.Firebase.ProjectID = projectID
	}

	// Signing keys envs
	if priv := os.Getenv("ED25519_PRIVATE_KEY"); priv != "" {
		cfg.Signing.Ed25519PrivateKeyBase64 = priv
	}
	if pub := os.Getenv("ED25519_PUBLIC_KEY"); pub != "" {
		cfg.Signing.Ed25519PublicKeyBase64 = pub
	}

	// Feature flags
	if f := os.Getenv("FEATURE_FRIENDS"); f != "" {
		cfg.Features.Friends = (f == "true" || f == "1")
	}
	if f := os.Getenv("FEATURE_WEEKLY_CHALLENGES"); f != "" {
		cfg.Features.WeeklyChallenges = (f == "true" || f == "1")
	}
	if f := os.Getenv("FEATURE_VOICE_EXPLANATIONS"); f != "" {
		cfg.Features.VoiceExplanations = (f == "true" || f == "1")
	}
	if f := os.Getenv("FEATURE_AI_MISTAKE_ANALYSIS"); f != "" {
		cfg.Features.AIMistakeAnalysis = (f == "true" || f == "1")
	}

	// Exam rules
	if qCount := os.Getenv("EXAM_QUESTION_COUNT"); qCount != "" {
		if val, err := strconv.Atoi(qCount); err == nil {
			cfg.Rules.ExamQuestionCount = val
		}
	}
	if dSec := os.Getenv("EXAM_DURATION_SEC"); dSec != "" {
		if val, err := strconv.Atoi(dSec); err == nil {
			cfg.Rules.ExamDurationSec = val
		}
	}
	if pCount := os.Getenv("EXAM_PASS_COUNT"); pCount != "" {
		if val, err := strconv.Atoi(pCount); err == nil {
			cfg.Rules.ExamPassCount = val
		}
	}

	// Setup signing keys
	if cfg.Signing.Ed25519PrivateKeyBase64 != "" {
		privBytes, err := base64.StdEncoding.DecodeString(cfg.Signing.Ed25519PrivateKeyBase64)
		if err == nil && len(privBytes) == ed25519.PrivateKeySize {
			cfg.Ed25519PrivKey = ed25519.PrivateKey(privBytes)
			cfg.Ed25519PubKey = cfg.Ed25519PrivKey.Public().(ed25519.PublicKey)
		}
	}

	// If no keys configured, generate a deterministic key for dev/testing
	if len(cfg.Ed25519PrivKey) == 0 {
		seed := make([]byte, ed25519.SeedSize)
		copy(seed, "avtofast-content-signing-seed-dev")
		cfg.Ed25519PrivKey = ed25519.NewKeyFromSeed(seed)
		cfg.Ed25519PubKey = cfg.Ed25519PrivKey.Public().(ed25519.PublicKey)
		cfg.Signing.Ed25519PublicKeyBase64 = base64.StdEncoding.EncodeToString(cfg.Ed25519PubKey)
	}

	return cfg, nil
}
