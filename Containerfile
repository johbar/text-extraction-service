FROM docker.io/golang

RUN apt-get update && apt-get upgrade -y && apt-get install -y libpoppler-glib-dev libcairo2-dev

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/text-extraction-service

CMD text-extraction-service