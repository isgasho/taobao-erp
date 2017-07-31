package main

import (
	"github.com/goushuyun/taobao-erp/db"
	"github.com/goushuyun/taobao-erp/pb"
	"github.com/wothing/worpc"
	"google.golang.org/grpc"

	"github.com/goushuyun/taobao-erp/book/service"
)

const (
	svcName = "users"
	port    = 10017
)

func main() {
	m := db.NewMicro(svcName, port)
	m.RegisterPG()
	m.RegisterRedis()

	s := grpc.NewServer(grpc.UnaryInterceptor(worpc.UnaryInterceptorChain(worpc.Recovery, worpc.Logging)))
	pb.RegisterBookServiceServer(s, &service.BookServer{})

	s.Serve(m.CreateListener())
}
