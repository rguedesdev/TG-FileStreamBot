package utils

import (
	"github.com/rguedesdev/TG-FileStreamBot/config"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
)

func PackFile(fileName string, fileSize int64, mimeType string, fileID int64) string {
	mac := hmac.New(sha256.New, []byte(config.ValueOf.BotToken))
	writeHashString(mac, fileName)
	_ = binary.Write(mac, binary.BigEndian, fileSize)
	writeHashString(mac, mimeType)
	_ = binary.Write(mac, binary.BigEndian, fileID)
	return hex.EncodeToString(mac.Sum(nil))
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func writeHashString(writer hashWriter, value string) {
	_ = binary.Write(writer, binary.BigEndian, uint64(len(value)))
	_, _ = writer.Write([]byte(value))
}

func GetShortHash(fullHash string) string {
	return fullHash[:config.ValueOf.HashLength]
}

func CheckHash(inputHash string, expectedHash string) bool {
	expected := GetShortHash(expectedHash)
	return len(inputHash) == len(expected) && subtle.ConstantTimeCompare([]byte(inputHash), []byte(expected)) == 1
}
