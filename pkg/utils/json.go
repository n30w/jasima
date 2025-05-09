package utils

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
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

func GenerateJsonSchema[T any]() ([]byte, error) {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	schema.ID = ""
	schema.Version = ""
	schema.AdditionalProperties = nil
	sc, err := json.Marshal(schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal JSON")
	}
	return sc, nil
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

func StructToMap(s any) (map[string]any, error) {
	var m map[string]any

	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}
