package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type StringSlice []string

// String returns the string representation of the URL.
//
//goland:noinspection GoMixedReceiverTypes
func (s StringSlice) String() string {
	return strings.Join(s, ",")
}

// MarshalText implements [encoding.TextMarshaler] interface.
//
//goland:noinspection GoMixedReceiverTypes
func (s StringSlice) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
//
//goland:noinspection GoMixedReceiverTypes
func (s *StringSlice) UnmarshalText(text []byte) error {
	*s = strings.Split(string(text), ",")

	return nil
}

// UnmarshalJSON implements the [json.Unmarshaler] interface.
//
//goland:noinspection GoMixedReceiverTypes
func (s *StringSlice) UnmarshalJSON(jsonBytes []byte) error {
	var slice []string

	err := json.NewDecoder(bytes.NewReader(jsonBytes)).Decode(&slice)

	*s = slice

	//nolint:wrapcheck
	return err
}

type Float64Slice []float64

// String returns the string representation of the URL.
//
//goland:noinspection GoMixedReceiverTypes
func (s Float64Slice) String() string {
	stringSlice := make([]string, len(s))

	for i, floatValue := range s {
		stringSlice[i] = strconv.FormatFloat(floatValue, 'g', -1, 64)
	}

	return strings.Join(stringSlice, ",")
}

// MarshalText implements [encoding.TextMarshaler] interface.
//
//goland:noinspection GoMixedReceiverTypes
func (s Float64Slice) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
//
//goland:noinspection GoMixedReceiverTypes
func (s *Float64Slice) UnmarshalText(text []byte) error {
	stringSlice := strings.Split(string(text), ",")
	floatSlice := make(Float64Slice, len(stringSlice))

	var err error

	for i, stringFloat := range stringSlice {
		floatSlice[i], err = strconv.ParseFloat(stringFloat, 64)
		if err != nil {
			return fmt.Errorf("failed to parse float64 from string '%s': %w", stringFloat, err)
		}
	}

	*s = floatSlice

	return nil
}

// UnmarshalJSON implements the [json.Unmarshaler] interface.
//
//goland:noinspection GoMixedReceiverTypes
func (s *Float64Slice) UnmarshalJSON(jsonBytes []byte) error {
	var slice []float64

	err := json.NewDecoder(bytes.NewReader(jsonBytes)).Decode(&slice)

	*s = slice

	//nolint:wrapcheck
	return err
}
