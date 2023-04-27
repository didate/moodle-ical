FROM golang:1.19-alpine

WORKDIR /app

COPY testdata/template.txt ./

# COPY .env ./

COPY go.sum ./
COPY go.mod ./

RUN go mod download

COPY *.go ./

RUN go build .

EXPOSE 8080

RUN mkdir ./ics

CMD [ "./go-calendar", "-t", "template.txt", "-d" ,"ics" ]
