package hw04lrucache

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	t.Run("empty cache", func(t *testing.T) {
		c := NewCache(10)

		_, ok := c.Get("aaa")
		require.False(t, ok)

		_, ok = c.Get("bbb")
		require.False(t, ok)
	})

	t.Run("simple", func(t *testing.T) {
		c := NewCache(5)

		wasInCache := c.Set("aaa", 100)
		require.False(t, wasInCache)

		wasInCache = c.Set("bbb", 200)
		require.False(t, wasInCache)

		val, ok := c.Get("aaa")
		require.True(t, ok)
		require.Equal(t, 100, val)

		val, ok = c.Get("bbb")
		require.True(t, ok)
		require.Equal(t, 200, val)

		wasInCache = c.Set("aaa", 300)
		require.True(t, wasInCache)

		val, ok = c.Get("aaa")
		require.True(t, ok)
		require.Equal(t, 300, val)

		val, ok = c.Get("ccc")
		require.False(t, ok)
		require.Nil(t, val)
	})

	t.Run("purge logic", func(t *testing.T) {
		c := NewCache(3)

		require.False(t, c.Set("first", 1))
		require.False(t, c.Set("second", 2))
		require.False(t, c.Set("third", 3))
		require.False(t, c.Set("fourth", 4))

		value, ok := c.Get("first")
		require.False(t, ok)
		require.Nil(t, value)

		for key, expected := range map[Key]int{"second": 2, "third": 3, "fourth": 4} {
			value, ok = c.Get(key)
			require.True(t, ok)
			require.Equal(t, expected, value)
		}
	})

	t.Run("least recently used item is purged", func(t *testing.T) {
		c := NewCache(3)
		c.Set("first", 1)
		c.Set("second", 2)
		c.Set("third", 3)

		value, ok := c.Get("first")
		require.True(t, ok)
		require.Equal(t, 1, value)
		require.True(t, c.Set("third", 30))
		require.False(t, c.Set("fourth", 4))

		value, ok = c.Get("second")
		require.False(t, ok)
		require.Nil(t, value)

		value, ok = c.Get("third")
		require.True(t, ok)
		require.Equal(t, 30, value)
	})

	t.Run("clear", func(t *testing.T) {
		c := NewCache(2)
		c.Set("first", 1)
		c.Set("second", 2)

		c.Clear()

		value, ok := c.Get("first")
		require.False(t, ok)
		require.Nil(t, value)
		value, ok = c.Get("second")
		require.False(t, ok)
		require.Nil(t, value)

		require.False(t, c.Set("third", 3))
		value, ok = c.Get("third")
		require.True(t, ok)
		require.Equal(t, 3, value)
	})

	t.Run("zero capacity", func(t *testing.T) {
		c := NewCache(0)

		require.False(t, c.Set("key", "value"))
		value, ok := c.Get("key")
		require.False(t, ok)
		require.Nil(t, value)
	})

	t.Run("nil value", func(t *testing.T) {
		c := NewCache(1)

		require.False(t, c.Set("", nil))
		value, ok := c.Get("")
		require.True(t, ok)
		require.Nil(t, value)

		require.True(t, c.Set("", "updated"))
		require.False(t, c.Set("another", nil))
		value, ok = c.Get("")
		require.False(t, ok)
		require.Nil(t, value)
	})
}

func TestCacheMultithreading(t *testing.T) {
	c := NewCache(10)
	wg := &sync.WaitGroup{}
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < 10_000; i++ {
			c.Set(Key(strconv.Itoa(i%100)), i)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10_000; i++ {
			c.Get(Key(strconv.Itoa(i % 100)))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1_000; i++ {
			c.Clear()
		}
	}()

	wg.Wait()

	require.False(t, c.Set("final", 42))
	value, ok := c.Get("final")
	require.True(t, ok)
	require.Equal(t, 42, value)
}
