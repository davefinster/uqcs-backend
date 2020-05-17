FROM golang:1.14.2-stretch
WORKDIR /root/project
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -v . && \
    cd migrate && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -v .

FROM ubuntu
WORKDIR /root/
COPY --from=0 /root/project/backend .
COPY --from=0 /root/project/migrate ./migrate
RUN curl -L -O https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.3.2/grpc_health_probe-linux-amd64 && \
		mv grpc_health_probe-linux-amd64 grpc_health_probe && \
		chmod +x grpc_health_probe
EXPOSE 8080
ENTRYPOINT ["/root/backend"]