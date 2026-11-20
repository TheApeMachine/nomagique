package runtime

import (
	"strings"

	"github.com/bytedance/sonic"
)

/*
Token is one emitted path letter plus its impulse brightness.
*/
type Token struct {
	Symbols    []string `json:"symbols"`
	Brightness float64  `json:"brightness"`
}

/*
NewToken copies symbols so path tokens on the envelope stay immutable across rings.
*/
func NewToken(symbols []string, brightness float64) Token {
	if len(symbols) == 0 {
		return Token{}
	}

	out := make([]string, len(symbols))
	copy(out, symbols)

	return Token{
		Symbols:    out,
		Brightness: brightness,
	}
}

/*
String renders one path segment for envelope.Paths JSON and dashboard facets.
*/
func (token Token) String() string {
	if len(token.Symbols) == 0 {
		return ""
	}

	if len(token.Symbols) == 1 {
		return token.Symbols[0]
	}

	return "[" + strings.Join(token.Symbols, "-") + "]"
}

/*
Tokens is the ordered path list on Envelope.Paths for solver and UI replay.
*/
type Tokens []Token

/*
NewTokens starts an empty path chain before cognition appends per tick.
*/
func NewTokens() Tokens {
	return make([]Token, 0)
}

/*
Append extends the path without mutating a shared slice backing store.
*/
func (tokens Tokens) Append(token Token) Tokens {
	return append(tokens, token)
}

/*
String joins path segments for logging and Publish("paths").
*/
func (tokens Tokens) String() string {
	parts := make([]string, 0, len(tokens))

	for _, t := range tokens {
		if s := t.String(); s != "" {
			parts = append(parts, s)
		}
	}

	return strings.Join(parts, "/")
}

/*
Brightness sums impulse along the path for desk tier gating and grid weighting.
*/
func (tokens Tokens) Brightness() float64 {
	brightness := 0.0

	for _, t := range tokens {
		brightness += t.Brightness
	}

	return brightness
}

/*
Len is path depth for trie and cognition bounds checks.
*/
func (tokens Tokens) Len() int {
	return len(tokens)
}

/*
Last is the active path head decisions and Emit read each tick.
*/
func (tokens Tokens) Last() Token {
	if len(tokens) == 0 {
		return Token{}
	}

	return tokens[len(tokens)-1]
}

/*
StringList exposes per-segment strings for UI tables without re-parsing paths.
*/
func (tokens Tokens) StringList() []string {
	out := make([]string, 0, len(tokens))

	for _, t := range tokens {
		out = append(out, t.String())
	}

	return out
}

/*
MarshalJSON stores symbols only so tick JSON stays compact in store uploads.
*/
func (token Token) MarshalJSON() ([]byte, error) {
	return sonic.Marshal(token.Symbols)
}

/*
UnmarshalJSON reloads path letters from persisted envelope ticks.
*/
func (token *Token) UnmarshalJSON(data []byte) error {
	return sonic.Unmarshal(data, &token.Symbols)
}
