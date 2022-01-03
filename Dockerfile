FROM golang

WORKDIR /app

COPY ["go.mod", "go.sum", "/app/"]

RUN go mod download

COPY ["main.go", "args.go", "/app/"]
COPY "templates" "/app/templates"

RUN ls -alh

ENTRYPOINT ["go", "run", "."]





