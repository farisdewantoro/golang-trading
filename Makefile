CONTAINER_NAME ?= golang-trading:asia-southeast2:postgres-id
PORT ?= 5435

run-db-proxy:
	@echo "Running Cloud SQL Proxy for container: $(CONTAINER_NAME) on port: $(PORT)"
	cloud-sql-proxy $(CONTAINER_NAME) -p $(PORT)