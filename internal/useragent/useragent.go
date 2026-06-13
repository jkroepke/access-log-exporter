package useragent

import (
	"sync"

	"github.com/ua-parser/uap-go/uaparser"
)

//nolint:gochecknoglobals // user agent parser is a global singleton
var parser = sync.OnceValue(func() *uaparser.Parser {
	parser, err := uaparser.New()
	if err != nil {
		panic(err)
	}

	return parser
})

func New() *uaparser.Parser {
	return parser()
}
