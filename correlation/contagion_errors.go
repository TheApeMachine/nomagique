package correlation

type ContagionErrorType string

const (
	ContagionErrorNilReceiver           ContagionErrorType = "require non-nil contagion stage"
	ContagionErrorInsufficientSnapshots ContagionErrorType = "require member interval snapshots"
)

type ContagionError string

func (contagionError ContagionError) Error() string {
	return string(contagionError)
}
