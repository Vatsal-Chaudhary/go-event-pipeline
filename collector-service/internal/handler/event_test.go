package handler

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestExtractIPAddress_PrioritizesXForwardedFor(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(extractIPAddress(c))
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.11, 198.51.100.1")
	req.Header.Set("X-Real-IP", "198.51.100.99")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if got, want := string(body), "203.0.113.11"; got != want {
		t.Fatalf("extractIPAddress() = %q, want %q", got, want)
	}
}

func TestExtractIPAddress_FallsBackToXRealIP(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(extractIPAddress(c))
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "198.51.100.42")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if got, want := string(body), "198.51.100.42"; got != want {
		t.Fatalf("extractIPAddress() = %q, want %q", got, want)
	}
}
