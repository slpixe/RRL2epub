
#build stage
FROM golang:alpine AS builder
WORKDIR /go/src/app
COPY . .
RUN apk add --no-cache git
# RUN go-wrapper download   # "go get -d -v ./..."
# RUN go-wrapper install    # "go install -v ./..."
RUN go get -d -v ./...
RUN go install -v ./...

#final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/bin/app /app
# ENTRYPOINT ./app wn:7176992105000305
# ENTRYPOINT ./app wn:10766915905158505
LABEL Name=rrl2epub Version=0.0.1
EXPOSE 3000