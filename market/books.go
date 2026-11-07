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

func NewBooks() *Books {
	return &Books{
		books: make(map[string]*Book),
	}
}

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

func (books *Books) Get(symbol string) *Book {
	books.mu.RLock()
	defer books.mu.RUnlock()
	return books.books[strings.ToUpper(symbol)]
}

func (books *Books) Symbols() []string {
	books.mu.RLock()
	defer books.mu.RUnlock()

	out := make([]string, 0, len(books.books))

	for symbol := range books.books {
		out = append(out, symbol)
	}

	return out
}
