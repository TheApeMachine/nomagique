package market

import (
	"strings"
	"sync"
)

/*
Books manages per-symbol order books.
*/
type Books struct {
	mu    sync.RWMutex
	books map[string]*Book
}

/*
NewBooks creates the symbol→book map virtual tiers seed before live L3 arrives.
*/
func NewBooks() *Books {
	return &Books{
		books: make(map[string]*Book),
	}
}

/*
Create registers one symbol book with depth cap for broker Expect/Wait boot.
*/
func (books *Books) Create(symbol string, depth int) *Book {
	books.mu.Lock()
	defer books.mu.Unlock()

	name := strings.ToUpper(symbol)
	book := NewBook()
	book.Name = name
	book.EnableMaxDepth = true
	book.MaxDepth = depth
	books.books[name] = book
	return book
}

/*
Get returns the lane-local book signals read without cross-symbol locking.
*/
func (books *Books) Get(symbol string) *Book {
	books.mu.RLock()
	defer books.mu.RUnlock()
	return books.books[strings.ToUpper(symbol)]
}

/*
Symbols lists subscribed books for desk fan-out and invalidation callbacks.
*/
func (books *Books) Symbols() []string {
	books.mu.RLock()
	defer books.mu.RUnlock()

	out := make([]string, 0, len(books.books))

	for symbol := range books.books {
		out = append(out, symbol)
	}

	return out
}
