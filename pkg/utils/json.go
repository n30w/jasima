package utils

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// GenerateSchema uses jsonschema library to make a JSON schema.
// Retrieved from:
// https://github.com/openai/openai-go/blob/main/examples/structured-outputs/main.go
func GenerateSchema[T any]() any {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

func Unmarshal[T any](s string) (
	T,
	error,
) {
	var d T
	err := json.Unmarshal(
		[]byte(s),
		&d,
	)
	if err != nil {
		return d, err
	}

	return d, nil
}
