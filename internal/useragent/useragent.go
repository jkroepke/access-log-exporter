package useragent

import (
	"sync"

	"github.com/ua-parser/uap-go/uaparser"
)

//nolint:gochecknoglobals // user agent parser is a global singleton
var parser = sync.OnceValue(func() *uaparser.Parser {
	return uaparser.NewFromSaved()
})

func New() *uaparser.Parser {
	return parser()
}
