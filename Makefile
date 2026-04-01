BINARY := kli
NAMESPACE ?= default

.PHONY: build test-apply test-undeploy test-status test-logs clean

build:
	go build -o $(BINARY) .

## Manual apply tests — requires a running cluster (~/.kube/config)
test-apply: build
	./$(BINARY) apply -f testdata/deployment.yaml -n $(NAMESPACE)
	./$(BINARY) apply -f testdata/service.yaml -n $(NAMESPACE)
	./$(BINARY) apply -f testdata/multi-doc.yaml -n $(NAMESPACE)
	./$(BINARY) apply -f testdata/ -n $(NAMESPACE)

## Manual undeploy tests — requires a running cluster (~/.kube/config)
test-undeploy: build
	./$(BINARY) undeploy -f testdata/deployment.yaml -n $(NAMESPACE)
	./$(BINARY) undeploy -f testdata/service.yaml -n $(NAMESPACE)
	./$(BINARY) undeploy -f testdata/multi-doc.yaml -n $(NAMESPACE)

POD ?= <pod-name>

## Manual status test — requires a running cluster (~/.kube/config)
test-status: build
	./$(BINARY) status -n $(NAMESPACE)

## Manual logs tests — requires a running cluster and POD=<pod-name>
## Usage: make test-logs POD=nginx-xxx-yyy NAMESPACE=kli1
test-logs: build
	./$(BINARY) logs $(POD) -n $(NAMESPACE)
	./$(BINARY) logs $(POD) -n $(NAMESPACE) --grep notice
	./$(BINARY) logs $(POD) -n $(NAMESPACE) --follow --grep notice

clean:
	rm -f $(BINARY)
