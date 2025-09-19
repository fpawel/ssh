package config

import (
	"fmt"
	"net/url"
	"strings"
)

type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	KeyFile  string
}

func (c Config) What() string {
	s := c.Host
	if c.Port != "" && c.Port != "22" {
		s += ":" + c.Port
	}
	if c.Username != "" {
		s = c.Username + "@" + s
	}
	return s
}

func ParseConnectionString(connStr string) (Config, error) {
	// Добавляем префикс, если его нет
	if !strings.HasPrefix(connStr, "ssh://") {
		connStr = "ssh://" + connStr
	}

	u, err := url.Parse(connStr)
	if err != nil {
		return Config{}, fmt.Errorf("ошибка парсинга URL: %w", err)
	}

	if u.User == nil {
		return Config{}, fmt.Errorf("отсутствует имя пользователя")
	}

	username := u.User.Username()
	password, _ := u.User.Password()

	hostname := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "22" // по умолчанию
	}

	if hostname == "" {
		return Config{}, fmt.Errorf("отсутствует хост")
	}

	return Config{
		Username: username,
		Password: password,
		Host:     hostname,
		Port:     port,
	}, nil
}
