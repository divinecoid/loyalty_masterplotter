package config

import "testing"

func TestValidate(t *testing.T) {
	good := Config{Production: true, JWTSecret: "0123456789abcdef0123456789abcdef", CookieSecure: true,
		PublicURL: "https://loyalty.example.com", CORSOrigins: "https://loyalty.example.com",
		DatabaseURL: "postgres://app:s3cret@db:5432/app", AdminPassword: "a-long-password-1"}
	if err := good.Validate(); err != nil {
		t.Fatalf("config baik ditolak: %v", err)
	}
	if err := (Config{JWTSecret: devSecret}).Validate(); err != nil {
		t.Fatalf("dev harus lolos: %v", err)
	}
	bad := []func(*Config){
		func(c *Config) { c.JWTSecret = devSecret },
		func(c *Config) { c.JWTSecret = "short" },
		func(c *Config) { c.CookieSecure = false },
		func(c *Config) { c.PublicURL = "http://x.com" },
		func(c *Config) { c.CORSOrigins = "*" },
		func(c *Config) { c.DatabaseURL = "postgres://loyalty:loyalty@db/x" },
		func(c *Config) { c.AdminPassword = "short" },
	}
	for i, f := range bad {
		c := good
		f(&c)
		if c.Validate() == nil {
			t.Errorf("kasus %d seharusnya ditolak", i)
		}
	}
}
