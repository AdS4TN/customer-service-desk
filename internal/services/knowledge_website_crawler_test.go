package services

import "testing"

func TestValidateWebsiteURL(t *testing.T) {
	got, host, err := validateWebsiteURL("https://www.example.com/products?source=test#top")
	if err != nil || got != "https://www.example.com/products" || host != "example.com" {
		t.Fatalf("validateWebsiteURL() = %q, %q, %v", got, host, err)
	}
	if _, _, err := validateWebsiteURL("http://127.0.0.1/internal"); err == nil {
		t.Fatal("loopback URL should be rejected")
	}
}
