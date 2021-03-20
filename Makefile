all:
	go build && go build cmd/slamdunk/main.go
	mv ./main ./slamdunk

clean:
	rm -f slamdunk
