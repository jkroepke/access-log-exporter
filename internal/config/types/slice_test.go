package types_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/config/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestSliceUnmarshalText(t *testing.T) {
	t.Parallel()

	slice := types.StringSlice{}

	require.NoError(t, slice.UnmarshalText([]byte("a,b,c,d")))

	assert.Equal(t, types.StringSlice{"a", "b", "c", "d"}, slice)
}

func TestSliceMarshalText(t *testing.T) {
	t.Parallel()

	slice, err := types.StringSlice{"a", "b", "c", "d"}.MarshalText()

	require.NoError(t, err)

	assert.Equal(t, []byte("a,b,c,d"), slice)
}

func TestSliceUnmarshalJSON(t *testing.T) {
	t.Parallel()

	slice := types.StringSlice{}

	require.NoError(t, json.NewDecoder(strings.NewReader(`["a","b","c","d"]`)).Decode(&slice))

	assert.Equal(t, types.StringSlice{"a", "b", "c", "d"}, slice)
}

func TestSliceUnmarshalYAML(t *testing.T) {
	t.Parallel()

	slice := types.StringSlice{}

	require.NoError(t, yaml.NewDecoder(strings.NewReader("- a\n- b\n- c\n- d\n")).Decode(&slice))

	assert.Equal(t, types.StringSlice{"a", "b", "c", "d"}, slice)
}

func TestFloat64SliceUnmarshalText(t *testing.T) {
	t.Parallel()

	slice := types.Float64Slice{}

	require.NoError(t, slice.UnmarshalText([]byte("0.5,0.6,0.7,0.8")))

	assert.Equal(t, types.Float64Slice{0.5, 0.6, 0.7, 0.8}, slice)
}

func TestFloat64SliceMarshalText(t *testing.T) {
	t.Parallel()

	slice, err := types.Float64Slice{0.5, 0.6, 0.7, 0.8}.MarshalText()

	require.NoError(t, err)

	assert.Equal(t, []byte("0.5,0.6,0.7,0.8"), slice)
}

func TestFloat64SliceUnmarshalJSON(t *testing.T) {
	t.Parallel()

	var slice types.Float64Slice

	require.NoError(t, json.NewDecoder(strings.NewReader(`[0.5,0.6,0.7,0.8]`)).Decode(&slice))

	assert.Equal(t, types.Float64Slice{0.5, 0.6, 0.7, 0.8}, slice)
}

func TestFloat64SliceUnmarshalYAML(t *testing.T) {
	t.Parallel()

	var slice types.Float64Slice

	require.NoError(t, yaml.NewDecoder(strings.NewReader("- 0.5\n- 0.6\n- 0.7\n- 0.8\n")).Decode(&slice))

	assert.Equal(t, types.Float64Slice{0.5, 0.6, 0.7, 0.8}, slice)
}
