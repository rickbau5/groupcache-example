objects = proto/groupcache_grpc.pb.go proto/groupcache_grpc_grpc.pb.go

build-proto: proto-deps $(objects)

proto-deps: proto/github.com/mailgun/groupcache/groupcachepb/groupcache.proto

$(objects):
	protoc --go_out=proto/ --go_opt=paths=source_relative \
		--go-grpc_out=proto/ --go-grpc_opt=paths=source_relative \
		-I proto/. \
		--go_opt=Mgithub.com/mailgun/groupcache/groupcachepb/groupcache.proto=github.com/mailgun/groupcache/groupcachepb \
		proto/groupcache_grpc.proto
	-@rm -r proto/github.com/

proto/github.com/mailgun/groupcache/groupcachepb/groupcache.proto: proto/github.com/mailgun/groupcache/groupcachepb
	cp vendor/github.com/mailgun/groupcache/groupcachepb/groupcache.proto $@

proto/github.com/mailgun/groupcache/groupcachepb:
	mkdir -p $@

.clean-proto:
	-@rm proto/groupcache_grpc.pb.go proto/groupcache_grpc_grpc.pb.go