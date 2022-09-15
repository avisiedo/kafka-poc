package schema

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

const (
	SchemaHeaderKey  = "header"
	SchemaRequestKey = "request"
)

// https://pkg.go.dev/embed

//go:embed "header.message.json"
var schemaHeader string

//go:embed "introspectRequest.message.json"
var schemaMessageRequest string

type SchemaMap map[string](*gojsonschema.Schema)

func newSchema(data []byte) (*gojsonschema.Schema, error) {
	var (
		schema *gojsonschema.Schema
		body   interface{}
		err    error
	)
	if err = json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("[newSchema] error unmarshalling: %w", err)
	}
	loader := gojsonschema.NewGoLoader(body)
	if schema, err = gojsonschema.NewSchema(loader); err != nil {
		return nil, fmt.Errorf("[newSchema] error creating schema: %w", err)
	}
	return schema, nil
}

// UnmarshallSchemas
func UnmarshallSchemas(schemas *SchemaMap) error {
	var (
		output SchemaMap = SchemaMap{}
		schema *gojsonschema.Schema
		err    error
	)

	if schemas == nil {
		return fmt.Errorf("[UnmarshallSchemas] schemas cannot be nil")
	}

	if schema, err = newSchema([]byte(schemaHeader)); err != nil {
		return fmt.Errorf("[UnmarshallSchemas] error creating schema for %s: %w", SchemaHeaderKey, err)
	}
	output[SchemaHeaderKey] = schema

	if schema, err = newSchema([]byte(schemaMessageRequest)); err != nil {
		return fmt.Errorf("[UnmarshallSchemas] error creating schema for %s: %w", schemaMessageRequest, err)
	}
	output[SchemaRequestKey] = schema

	*schemas = output
	return nil
}

func ValidateWithSchemaAndInterface(schema *gojsonschema.Schema, body interface{}) error {
	if schema == nil {
		return fmt.Errorf("[ValidateSchema] schema is nil")
	}
	reference := gojsonschema.NewGoLoader(body)
	if _, err := schema.Validate(reference); err != nil {
		return fmt.Errorf("[ValidateSchema] schema invalid: %w", err)
	}
	return nil
}
