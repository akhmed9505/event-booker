package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Http       HttpConfig
	AuthConfig AuthConfig
	Postgres   DBConfig
	RabbitMQ   RabbitMQConfig
	TelegBot   TelegramBotConfig
	EmailSmpt  EmailChannel
}

type HttpConfig struct {
	Port            string        `env:"HTTP_PORT"`
	ReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT"`
	WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT"`
}

type AuthConfig struct {
	AccessTokenTTL  time.Duration `env:"ACCESS_TTL" env-default:"15m"`
	RefreshTokenTTL time.Duration `env:"REFRESH_TTL" env-default:"720h"`
	PasswordSalt    string        `env:"PASSWORD_SALT" env-required:"true"`
	JWTSigningKey   string        `env:"JWT_SIGNING_KEY" env-required:"true"`
}
type PostgresConfig struct {
	Host     string `env:"POSTGRES_HOST"`
	Port     int    `env:"POSTGRES_PORT"`
	Database string `env:"POSTGRES_DATABASE"`
	User     string `env:"POSTGRES_USER"`
	Password string `env:"POSTGRES_PASSWORD"`
	SSLMode  string `env:"POSTGRES_SSL_MODE"`
}

type RabbitMQConfig struct {
	Host     string `env:"RABBIT_HOST"`
	Port     int    `env:"RABBIT_PORT"`
	User     string `env:"RABBIT_USER"`
	Password string `env:"RABBIT_PASSWORD"`
}

type EmailChannel struct {
	SmptPort     int    `env:"SMPT_PORT"`
	SmptServer   string `env:"SMPT_SERVER"`
	SmptEmail    string `env:"SMPT_EMAIL"`
	SmptPassword string `env:"SMPT_PASSWORD"`
}
type TelegramBotConfig struct {
	Key    string `env:"TELEGRAMBOT_KEY"`
	ChatID int64  `env:"TELEGRAMBOT_CHATID"`
}

type DBConfig struct {
	Master PostgresConfig
	Slaves []PostgresConfig

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or could not be loaded, continuing with system env variables: %v", err)
	}

	var cfg Config

	cfg.Http.Port = os.Getenv("HTTP_PORT")

	readTimeout := os.Getenv("HTTP_READ_TIMEOUT")
	if readTimeout == "" {
		readTimeout = "10s"
	}
	cfg.Http.ReadTimeout, _ = time.ParseDuration(readTimeout)

	writeTimeout := os.Getenv("HTTP_WRITE_TIMEOUT")
	if writeTimeout == "" {
		writeTimeout = "10s"
	}
	cfg.Http.WriteTimeout, _ = time.ParseDuration(writeTimeout)

	shutdownTimeout := os.Getenv("HTTP_SHUTDOWN_TIMEOUT")
	if shutdownTimeout == "" {
		shutdownTimeout = "10s"
	}
	cfg.Http.ShutdownTimeout, _ = time.ParseDuration(shutdownTimeout)

	cfg.Postgres.Master.Host = os.Getenv("POSTGRES_HOST")
	cfg.Postgres.Master.Port, _ = strconv.Atoi(os.Getenv("POSTGRES_PORT"))
	cfg.Postgres.Master.Database = os.Getenv("POSTGRES_DATABASE")
	cfg.Postgres.Master.User = os.Getenv("POSTGRES_USER")
	cfg.Postgres.Master.Password = os.Getenv("POSTGRES_PASSWORD")
	cfg.Postgres.Master.SSLMode = os.Getenv("POSTGRES_SSL_MODE")
	cfg.AuthConfig.PasswordSalt = os.Getenv("PASSWORD_SALT")
	cfg.AuthConfig.JWTSigningKey = os.Getenv("JWT_SIGNING_KEY")
	accessTTLStr := os.Getenv("ACCESS_TTL")
	refreshTTLStr := os.Getenv("REFRESH_TTL")

	cfg.RabbitMQ.Host = os.Getenv("RABBIT_HOST")
	cfg.RabbitMQ.Port, _ = strconv.Atoi(os.Getenv("RABBIT_PORT"))
	cfg.RabbitMQ.User = os.Getenv("RABBIT_USER")
	cfg.RabbitMQ.Password = os.Getenv("RABBIT_PASSWORD")

	cfg.TelegBot.Key = os.Getenv("TELEGRAMBOT_KEY")
	cfg.TelegBot.ChatID, _ = strconv.ParseInt(os.Getenv("TELEGRAMBOT_CHATID"), 10, 64)

	cfg.EmailSmpt.SmptPort, _ = strconv.Atoi(os.Getenv("SMPT_PORT"))
	cfg.EmailSmpt.SmptPassword = os.Getenv("SMPT_PASSWORD")
	cfg.EmailSmpt.SmptEmail = os.Getenv("SMPT_EMAIL")
	cfg.EmailSmpt.SmptServer = os.Getenv("SMPT_SERVER")
	accessTTL, err := time.ParseDuration(accessTTLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ACCESS_TTL: %w", err)
	}
	refreshTTL, err := time.ParseDuration(refreshTTLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid REFRESH_TTL: %w", err)
	}

	cfg.AuthConfig.AccessTokenTTL = accessTTL
	cfg.AuthConfig.RefreshTokenTTL = refreshTTL
	// Отладочный вывод всех значений
	log.Printf("Config loaded: %+v\n", cfg)

	return &cfg, nil
}
