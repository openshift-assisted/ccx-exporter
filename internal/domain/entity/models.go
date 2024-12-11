package entity

type Event struct {
	Name     string                 `json:"name"`
	Payload  interface{}            `json:"payload"`
	Metadata map[string]interface{} `json:"metadata"`
}
