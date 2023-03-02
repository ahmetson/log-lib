package data_type

import (
	"container/list"
)

type Queue struct {
	l      *list.List
	length int
}

func NewQueue() *Queue {
	return &Queue{
		length: 1000,
		l:      list.New(),
	}
}

func (q *Queue) Len() int {
	return q.l.Len()
}

func (q *Queue) IsEmpty() bool {
	return q.l.Len() == 0
}

func (q *Queue) IsFull() bool {
	return q.l.Len() == q.length
}

func (q *Queue) Push(item interface{}) {
	if q.IsFull() {
		q.Pop()
	}
	q.l.PushBack(item)
}

func (q *Queue) Pop() interface{} {
	return q.l.Remove(q.l.Back())
}
