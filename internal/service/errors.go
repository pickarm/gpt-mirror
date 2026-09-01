package service

import "errors"

var ErrProviderNotConfigured = errors.New("provider is not configured; legacy gateway integration has been removed")
