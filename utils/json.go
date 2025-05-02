package utils

import "github.com/invopop/jsonschema"

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
