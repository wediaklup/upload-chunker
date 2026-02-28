all: uploadchunkerd
uploadchunkerd:
	go build -o uploadchunkerd .

.PHONY: clean
clean:
	rm -f uploadchunkerd
