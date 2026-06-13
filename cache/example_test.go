package cache_test

import (
	"fmt"

	"github.com/andyle182810/gframework/cache"
)

func ExampleBuildKey() {
	key := cache.BuildKey("user", 42, "profile")
	fmt.Println(key)
	// Output: user:42:profile
}

func ExampleBuildHashedKey() {
	// Use a hashed key when parts may contain separator characters or
	// unbounded user input — the result is a fixed-length hex digest.
	key := cache.BuildHashedKey("search", "alice@example.com", "page=1")
	fmt.Println(len(key))
	// Output: 64
}
