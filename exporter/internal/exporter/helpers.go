package exporter

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"

	"github.com/xuri/excelize/v2"
)

// rowWeight returns approximate row weight in bytes.
//
// Value lengths are calculated as follows:
// integers, double — 8 bytes;
// boolean — 1 byte;
// string — it's length;
// nil — 0 bytes.
func rowWeight(row []any) int {
	weight := 0
	for _, v := range row {
		if v == nil {
			continue
		}
		cell := v.(excelize.Cell)
		switch t := cell.Value.(type) {
		case int8, uint8, int16, uint16, int, uint, int32, uint32, int64, uint64, float64:
			weight += 8
		case bool:
			weight += 1
		case string:
			weight += len(t)
		case []byte:
			weight += len(t)
		}
	}
	return weight
}

// randomName returns 8 random bytes in hex.
func randomName() string {
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}

var alphanumRegex *regexp.Regexp

func init() {
	alphanumRegex, _ = regexp.Compile("[^a-zA-Z0-9_]")
}

func replaceNonAlphanumeric(in string) string {
	return alphanumRegex.ReplaceAllString(in, "_")
}
