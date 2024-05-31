package exporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestLimitedWriter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []any
		weight int
	}{
		{
			name: "numbers",
			values: []any{
				int8(0), uint8(0), int16(0), uint16(0), int32(0), uint32(0), int64(0), uint64(0),
				0, uint(0), 0.0,
			},
			weight: 8 * 11,
		},
		{
			name:   "bool",
			values: []any{true},
			weight: 1,
		},
		{
			name:   "string",
			values: []any{"hello"},
			weight: 5,
		},
		{
			name:   "bytes",
			values: []any{[]byte("hello")},
			weight: 5,
		},
		{
			name:   "nil",
			values: []any{nil, nil, nil},
			weight: 0,
		},
		{
			name:   "simple",
			values: []any{false, 42, 0.5, "hello", []byte("test"), nil},
			weight: 1 + 8 + 8 + 5 + 4 + 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := make([]any, len(tc.values))
			for i, v := range tc.values {
				if v == nil {
					row[i] = nil
				} else {
					row[i] = excelize.Cell{Value: v}
				}
			}
			require.Equal(t, tc.weight, rowWeight(row))
		})
	}
}
