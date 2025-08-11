package config_test

import (
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		conf config.Config
		err  string
	}{
		{
			config.Config{},
			"",
		},
	} {
		t.Run(tc.err, func(t *testing.T) {
			t.Parallel()

			err := config.Validate(tc.conf)
			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)

				if tc.err != "-" {
					assert.EqualError(t, err, tc.err)
				}
			}
		})
	}
}
