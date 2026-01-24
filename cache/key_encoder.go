package cache

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
)

var ErrCacheInvalidKeyType = errors.New("cache: invalid key type")

type KeyEncoder interface {
	Encode(key any) (string, error)
}

type StringKeyEncoder struct{}

func NewStringKeyEncoder() *StringKeyEncoder {
	return &StringKeyEncoder{}
}

func (e *StringKeyEncoder) Encode(key any) (string, error) {
	str, ok := key.(string)
	if !ok {
		return "", fmt.Errorf("%w (expected string, got %T)", ErrCacheInvalidKeyType, key)
	}

	return str, nil
}

type IntKeyEncoder struct{}

func NewIntKeyEncoder() *IntKeyEncoder {
	return &IntKeyEncoder{}
}

func (e *IntKeyEncoder) Encode(key any) (string, error) {
	num, ok := key.(int)
	if !ok {
		return "", fmt.Errorf("%w (expected int, got %T)", ErrCacheInvalidKeyType, key)
	}

	return strconv.Itoa(num), nil
}

type UUIDKeyEncoder struct{}

func NewUUIDKeyEncoder() *UUIDKeyEncoder {
	return &UUIDKeyEncoder{}
}

func (e *UUIDKeyEncoder) Encode(key any) (string, error) {
	id, ok := key.(uuid.UUID)
	if !ok {
		return "", fmt.Errorf("%w (expected uuid.UUID, got %T)", ErrCacheInvalidKeyType, key)
	}

	return id.String(), nil
}
