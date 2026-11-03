package market

import "encoding/json"

type Execution struct {
	Executions []*Execution `json:"executions"`
}

func NewExecution(data map[string]any) *Execution {
	return &Execution{
		Executions: data["executions"].([]*Execution),
	}
}

func (execution *Execution) MarshalJSON() ([]byte, error) {
	type alias Execution
	return json.Marshal((*alias)(execution))
}

func (execution *Execution) UnmarshalJSON(data []byte) error {
	type alias Execution
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*execution = Execution(decoded)
	return nil
}

type Executions []*Execution

func NewExecutions(data map[string]any) Executions {
	executions := make(Executions, 0)

	for _, execution := range data["executions"].([]*Execution) {
		executions = append(executions, execution)
	}

	return executions
}

func (executions *Executions) MarshalJSON() ([]byte, error) {
	type alias Executions
	return json.Marshal((*alias)(executions))
}

func (executions *Executions) UnmarshalJSON(data []byte) error {
	type alias Executions
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*executions = Executions(decoded)
	return nil
}
