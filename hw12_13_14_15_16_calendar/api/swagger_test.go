package api_test

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPISpecification(t *testing.T) {
	document, err := openapi3.NewLoader().LoadFromFile("swagger.json")
	if err != nil {
		t.Fatalf("load OpenAPI specification: %v", err)
	}
	if err = document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI specification: %v", err)
	}
}
