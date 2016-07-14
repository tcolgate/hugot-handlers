
all: docker

dist:
	mkdir dist

dist/prometheus:
	go get github.com/prometheus/prometheus/cmd/prometheus
	cp $$GOPATH/bin/prometheus ./dist

dist/alertmanager:
	go get github.com/prometheus/alertmanager
	cp $$GOPATH/bin/alertmanager ./dist

dist/hugot-demo: main.go
	go build -o dist/hugot-demo .

.PHONY: docker

docker: dist/prometheus dist/alertmanager dist/hugot-demo
	docker build .

