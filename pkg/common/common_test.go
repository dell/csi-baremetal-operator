package common

import (
	"testing"

	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/stretchr/testify/assert"
)

func Test_MatchLogLevel(t *testing.T) {
	tests := []struct {
		level    components.Level
		expected string
	}{
		{
			level:    components.InfoLevel,
			expected: "info",
		},
		{
			level:    components.DebugLevel,
			expected: "debug",
		},
		{
			level:    components.TraceLevel,
			expected: "trace",
		},
		{
			level:    "unknown",
			expected: "info",
		},
	}

	for _, tt := range tests {
		t.Run("Check log level "+string(tt.level), func(t *testing.T) {
			assert.Equal(t, string(tt.expected), MatchLogLevel(tt.level))
		})
	}
}

func Test_MatchFormat(t *testing.T) {
	tests := []struct {
		format         components.Format
		expectedFormat string
	}{
		{
			format:         components.JSONFormat,
			expectedFormat: "json",
		},
		{
			format:         components.TextFormat,
			expectedFormat: "text",
		},
		{
			format:         "unknown",
			expectedFormat: "text",
		},
	}

	for _, tt := range tests {
		t.Run("Check formats "+string(tt.format), func(t *testing.T) {
			assert.Equal(t, string(tt.expectedFormat), MatchLogFormat(tt.format))
		})
	}
}
