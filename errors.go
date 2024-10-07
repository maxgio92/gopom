package gopom

import (
	"github.com/pkg/errors"
)

var (
	ErrDepNotFound   = errors.New("dependency not found")
	ErrDepCacheEmpty = errors.New("dependency cache empty")
)
