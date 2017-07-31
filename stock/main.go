package main

import (
	"github.com/goushuyun/taobao-erp/db"
	"github.com/goushuyun/taobao-erp/pb"
	"github.com/wothing/worpc"
	"google.golang.org/grpc"

	"github.com/goushuyun/taobao-erp/stock/service"
)

const (
	svcName = "stock"
	port    = 10017
)

func main() {
	m := db.NewMicro(svcName, port)
	m.RegisterPG()

	s := grpc.NewServer(grpc.UnaryInterceptor(worpc.UnaryInterceptorChain(worpc.Recovery, worpc.Logging)))
	pb.RegisterStockServiceServer(s, &service.StockServer{})
	s.Serve(m.CreateListener())
}
