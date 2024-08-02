FROM golang:1.21.12

WORKDIR /neo

COPY . ./

RUN go mod download

CMD ["go", "test", "-v", "./..."]