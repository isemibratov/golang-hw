package hw04lrucache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		l := NewList()

		requireListState(t, l)
	})

	t.Run("push items", func(t *testing.T) {
		l := NewList()

		middle := l.PushFront(20)
		front := l.PushFront(10)
		back := l.PushBack(30)

		requireListState(t, l, 10, 20, 30)
		require.Same(t, front, l.Front())
		require.Same(t, back, l.Back())
		require.Same(t, middle, front.Next)
		require.Same(t, middle, back.Prev)
	})

	t.Run("remove items", func(t *testing.T) {
		l := NewList()
		front := l.PushBack(10)
		middle := l.PushBack(20)
		back := l.PushBack(30)

		l.Remove(middle)
		requireListState(t, l, 10, 30)
		require.Nil(t, middle.Prev)
		require.Nil(t, middle.Next)

		l.Remove(front)
		requireListState(t, l, 30)
		require.Nil(t, front.Prev)
		require.Nil(t, front.Next)

		l.Remove(back)
		requireListState(t, l)
		require.Nil(t, back.Prev)
		require.Nil(t, back.Next)
	})

	t.Run("move items to front", func(t *testing.T) {
		l := NewList()
		front := l.PushBack(10)
		middle := l.PushBack(20)
		back := l.PushBack(30)

		l.MoveToFront(middle)
		requireListState(t, l, 20, 10, 30)

		l.MoveToFront(middle)
		requireListState(t, l, 20, 10, 30)

		l.MoveToFront(back)
		requireListState(t, l, 30, 20, 10)
		require.Same(t, back, l.Front())
		require.Same(t, front, l.Back())
	})

	t.Run("complex", func(t *testing.T) {
		l := NewList()

		l.PushFront(10) // [10]
		l.PushBack(20)  // [10, 20]
		l.PushBack(30)  // [10, 20, 30]
		require.Equal(t, 3, l.Len())

		middle := l.Front().Next // 20
		l.Remove(middle)         // [10, 30]
		require.Equal(t, 2, l.Len())

		for i, v := range [...]int{40, 50, 60, 70, 80} {
			if i%2 == 0 {
				l.PushFront(v)
			} else {
				l.PushBack(v)
			}
		} // [80, 60, 40, 10, 30, 50, 70]

		require.Equal(t, 7, l.Len())
		require.Equal(t, 80, l.Front().Value)
		require.Equal(t, 70, l.Back().Value)

		l.MoveToFront(l.Front()) // [80, 60, 40, 10, 30, 50, 70]
		l.MoveToFront(l.Back())  // [70, 80, 60, 40, 10, 30, 50]

		elems := make([]int, 0, l.Len())
		for i := l.Front(); i != nil; i = i.Next {
			elems = append(elems, i.Value.(int))
		}
		require.Equal(t, []int{70, 80, 60, 40, 10, 30, 50}, elems)
	})
}

func requireListState(t *testing.T, l List, values ...interface{}) {
	t.Helper()

	require.Equal(t, len(values), l.Len())
	if len(values) == 0 {
		require.Nil(t, l.Front())
		require.Nil(t, l.Back())
		return
	}

	actual := make([]interface{}, 0, l.Len())
	var previous *ListItem
	for item := l.Front(); item != nil; item = item.Next {
		require.Same(t, previous, item.Prev)
		actual = append(actual, item.Value)
		previous = item
	}

	require.Equal(t, values, actual)
	require.Same(t, previous, l.Back())
	require.Nil(t, l.Front().Prev)
	require.Nil(t, l.Back().Next)

	reversed := make([]interface{}, 0, l.Len())
	var next *ListItem
	for item := l.Back(); item != nil; item = item.Prev {
		require.Same(t, next, item.Next)
		reversed = append(reversed, item.Value)
		next = item
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	require.Equal(t, values, reversed)
}
