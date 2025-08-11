package test

import (
	"io"
	"sync"

	"github.com/jkroepke/access-log-exporter/internal/config"
)

var DefaultConfig = sync.OnceValue(func() config.Config {
	conf, _ := config.New([]string{}, io.Discard)

	return conf
})
