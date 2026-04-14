OAPI_CODEGEN ?= /Users/hcchien/mirrormedia/bin/oapi-codegen
OPENAPI_SPEC ?= openapi.yaml
API_GEN ?= internal/api/api.gen.go

.PHONY: codegen test test-integration-pg test-integration-pg-pgx

codegen:
	@test -x "$(OAPI_CODEGEN)" || (echo "oapi-codegen not found at $(OAPI_CODEGEN)"; exit 1)
	$(OAPI_CODEGEN) -generate types,chi-server -package api -o $(API_GEN) $(OPENAPI_SPEC)
	gofmt -w $(API_GEN)

test:
	go test ./...

test-integration-pg:
	@test -n "$(LEITH_PG_TEST_DSN)" || (echo "LEITH_PG_TEST_DSN is required"; exit 1)
	go test -tags integration ./internal/store -run SQLStorePostgresIntegration -v

test-integration-pg-pgx:
	@test -n "$(LEITH_PG_TEST_DSN)" || (echo "LEITH_PG_TEST_DSN is required"; exit 1)
	go test -tags "integration pgxdriver" ./internal/store -run SQLStorePostgresIntegration -v
