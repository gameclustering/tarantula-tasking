package util

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/spaolacci/murmur3"
)

func EpochMillisecondsFromMidnight(year int, month int, day int) int64 {
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.UnixMilli()
}

func Partition(key []byte, pnumber uint32) uint32 {
	return murmur3.Sum32(key) % pnumber
}

func ToJson(obj any) []byte {
	data, err := json.Marshal(obj)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func Key(size int16) []byte {
	key := make([]byte, size)
	rand.Read(key)
	return key
}

func KeyToBase64(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

func KeyFromBase64(key string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(key)
}

func Hash32(key []byte) uint32 {
	return murmur3.Sum32(key)
}
