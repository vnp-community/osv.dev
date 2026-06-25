package asset

import "errors"

var (
    ErrInvalidIPAddress = errors.New("invalid IP address format")
    ErrAssetNotFound    = errors.New("asset not found")
)
