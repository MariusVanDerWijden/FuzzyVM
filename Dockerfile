FROM golang:latest as golang-builder 

RUN git clone https://github.com/mariusvanderwijden/fuzzyvm --depth 1

RUN cd fuzzyvm && go build

FROM golang:latest

COPY --from=golang-builder go/fuzzyvm/ /go/

ENTRYPOINT ["/bin/sh"]