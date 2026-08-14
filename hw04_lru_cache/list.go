package hw04lrucache

type List interface {
	Len() int
	Front() *ListItem
	Back() *ListItem
	PushFront(v interface{}) *ListItem
	PushBack(v interface{}) *ListItem
	Remove(i *ListItem)
	MoveToFront(i *ListItem)
}

type ListItem struct {
	Value interface{}
	Next  *ListItem
	Prev  *ListItem
}

type list struct {
	length int
	front  *ListItem
	back   *ListItem
}

func NewList() List {
	return new(list)
}

func (l *list) Len() int {
	return l.length
}

func (l *list) Front() *ListItem {
	return l.front
}

func (l *list) Back() *ListItem {
	return l.back
}

func (l *list) PushFront(v interface{}) *ListItem {
	item := &ListItem{Value: v, Next: l.front}
	if l.front == nil {
		l.back = item
	} else {
		l.front.Prev = item
	}

	l.front = item
	l.length++
	return item
}

func (l *list) PushBack(v interface{}) *ListItem {
	item := &ListItem{Value: v, Prev: l.back}
	if l.back == nil {
		l.front = item
	} else {
		l.back.Next = item
	}

	l.back = item
	l.length++
	return item
}

func (l *list) Remove(item *ListItem) {
	if item.Prev == nil {
		l.front = item.Next
	} else {
		item.Prev.Next = item.Next
	}

	if item.Next == nil {
		l.back = item.Prev
	} else {
		item.Next.Prev = item.Prev
	}

	item.Next = nil
	item.Prev = nil
	l.length--
}

func (l *list) MoveToFront(item *ListItem) {
	if item == l.front {
		return
	}

	item.Prev.Next = item.Next
	if item.Next == nil {
		l.back = item.Prev
	} else {
		item.Next.Prev = item.Prev
	}

	item.Prev = nil
	item.Next = l.front
	l.front.Prev = item
	l.front = item
}
