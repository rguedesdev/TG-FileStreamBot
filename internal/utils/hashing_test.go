package utils

import (
	"EverythingSuckz/fsb/config"
	"testing"
)

func TestPackFileUsesBotTokenAsHMACKey(t *testing.T) {
	oldToken := config.ValueOf.BotToken
	oldLength := config.ValueOf.HashLength
	t.Cleanup(func() {
		config.ValueOf.BotToken = oldToken
		config.ValueOf.HashLength = oldLength
	})

	config.ValueOf.HashLength = 24
	config.ValueOf.BotToken = "test-key-one"
	first := PackFile("video.mp4", 1234, "video/mp4", 5678)
	if len(first) != sha256HexLength {
		t.Fatalf("full HMAC length = %d, want %d", len(first), sha256HexLength)
	}
	if got := GetShortHash(first); len(got) != 24 {
		t.Fatalf("short HMAC length = %d, want 24", len(got))
	}

	config.ValueOf.BotToken = "test-key-two"
	second := PackFile("video.mp4", 1234, "video/mp4", 5678)
	if first == second {
		t.Fatal("HMAC did not change when the key changed")
	}
}

const sha256HexLength = 64
