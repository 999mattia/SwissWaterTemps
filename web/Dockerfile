FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod tidy 
COPY go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .


FROM alpine:latest  
RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/views ./views
COPY --from=builder /app/assets ./assets

EXPOSE 3000

CMD ["./main"]