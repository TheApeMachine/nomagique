package data

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/theapemachine/errnie"
)

/*
Attribute navigates into byte payloads using sonic SIMD path traversal
to pick out a typed attribute without unmarshaling the entire structure.
Handles both native JSON numbers/types and string-encoded venue numbers.
Returns the zero value of T if the attribute does not exist; a value that
cannot be converted is logged. Use AttributeOk to receive the error.
*/
func Attribute[T any](payload []byte, path ...any) T {
	val, err := AttributeOk[T](payload, path...)

	if err != nil && !errors.Is(err, ErrMissing) {
		errnie.Error(err)
	}

	return val
}

/*
ErrMissing reports that a path does not exist in a payload (or holds null).
*/
var ErrMissing = errors.New("data: attribute missing")

/*
AttributeOk picks the attribute at path out of payload as T.
A missing path returns ErrMissing.
*/
func AttributeOk[T any](payload []byte, path ...any) (T, error) {
	var zero T

	if len(payload) == 0 || len(path) == 0 {
		return zero, ErrMissing
	}

	root, err := sonic.Get(payload, path...)

	if errors.Is(err, ast.ErrNotExist) {
		return zero, ErrMissing
	}

	if err != nil {
		return zero, errnie.Err(
			errnie.UnprocessableContent,
			fmt.Sprintf("data: attribute %v unreadable", path),
			err,
		)
	}

	value, err := root.Interface()

	if err != nil {
		return zero, errnie.Err(
			errnie.UnprocessableContent,
			fmt.Sprintf("data: attribute %v unreadable", path),
			err,
		)
	}

	if value == nil {
		return zero, ErrMissing
	}

	if converted, ok := convertLookup[T](value); ok {
		return converted, nil
	}

	raw, err := root.Raw()

	if err != nil {
		return zero, errnie.Err(
			errnie.UnprocessableContent,
			fmt.Sprintf("data: attribute %v unreadable", path),
			err,
		)
	}

	var out T

	if err := sonic.UnmarshalString(raw, &out); err != nil {
		return zero, errnie.Err(
			errnie.UnprocessableContent,
			fmt.Sprintf("data: attribute %v not decodable as %T", path, zero),
			err,
		)
	}

	return out, nil
}

func convertLookup[T any](value any) (T, bool) {
	if typed, ok := value.(T); ok {
		return typed, true
	}

	if typed, ok := numericPeek[T](value); ok {
		return typed, true
	}

	return slicePeek[T](value)
}

func numericPeek[T any](value any) (T, bool) {
	var zero T

	switch source := value.(type) {
	case float64:
		if math.IsNaN(source) || math.IsInf(source, 0) {
			return zero, false
		}

		switch any(zero).(type) {
		case float64:
			return any(source).(T), true
		case int:
			if math.Trunc(source) == source {
				return any(int(source)).(T), true
			}
		case int64:
			if math.Trunc(source) == source {
				return any(int64(source)).(T), true
			}
		case string:
			return any(strconv.FormatFloat(source, 'f', -1, 64)).(T), true
		}

	case int:
		switch any(zero).(type) {
		case int:
			return any(source).(T), true
		case int64:
			return any(int64(source)).(T), true
		case float64:
			return any(float64(source)).(T), true
		case string:
			return any(strconv.Itoa(source)).(T), true
		}

	case int64:
		switch any(zero).(type) {
		case int64:
			return any(source).(T), true
		case int:
			return any(int(source)).(T), true
		case float64:
			return any(float64(source)).(T), true
		case string:
			return any(strconv.FormatInt(source, 10)).(T), true
		}

	case string:
		trimmed := strings.TrimSpace(source)
		if trimmed == "" {
			return zero, false
		}

		switch any(zero).(type) {
		case float64:
			if f, err := strconv.ParseFloat(trimmed, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
				return any(f).(T), true
			}
		case int:
			if i, err := strconv.Atoi(trimmed); err == nil {
				return any(i).(T), true
			} else if f, err := strconv.ParseFloat(trimmed, 64); err == nil && math.Trunc(f) == f {
				return any(int(f)).(T), true
			}
		case int64:
			if i, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
				return any(i).(T), true
			} else if f, err := strconv.ParseFloat(trimmed, 64); err == nil && math.Trunc(f) == f {
				return any(int64(f)).(T), true
			}
		case bool:
			if b, err := strconv.ParseBool(trimmed); err == nil {
				return any(b).(T), true
			}
		}

	case bool:
		switch any(zero).(type) {
		case bool:
			return any(source).(T), true
		case string:
			return any(strconv.FormatBool(source)).(T), true
		}
	}

	return zero, false
}

func slicePeek[T any](value any) (T, bool) {
	var zero T

	items, ok := value.([]any)
	if !ok {
		return zero, false
	}

	switch any(zero).(type) {
	case []string:
		values := make([]string, len(items))
		for index, item := range items {
			switch typed := item.(type) {
			case string:
				values[index] = typed
			case float64:
				values[index] = strconv.FormatFloat(typed, 'f', -1, 64)
			default:
				values[index] = fmt.Sprint(item)
			}
		}
		return any(values).(T), true

	case []float64:
		values := make([]float64, len(items))
		for index, item := range items {
			switch typed := item.(type) {
			case float64:
				values[index] = typed
			case int:
				values[index] = float64(typed)
			case int64:
				values[index] = float64(typed)
			case string:
				if f, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
					values[index] = f
				} else {
					return zero, false
				}
			default:
				return zero, false
			}
		}
		return any(values).(T), true
	}

	return zero, false
}
