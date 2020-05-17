buildProto:
	protoc -I proto/ proto/server.proto --go_out=plugins=grpc:./backend --go_opt=module=github.com/davefinster/uqcs-demo/backend
	