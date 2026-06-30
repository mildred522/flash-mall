package main

import (
	"log"
	"net"
	"os"

	"flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory/inventoryservice"
	"flash-mall/app/inventory/repository"
	"flash-mall/app/inventory/service"

	"github.com/cloudwego/kitex/server"
)

const defaultListenOn = "0.0.0.0:8093"

func main() {
	listenOn := os.Getenv("INVENTORY_LISTEN_ON")
	if listenOn == "" {
		listenOn = defaultListenOn
	}
	addr, err := net.ResolveTCPAddr("tcp", listenOn)
	if err != nil {
		log.Fatalf("resolve inventory listen addr %q: %v", listenOn, err)
	}

	inventoryService := service.New(repository.NewMemoryStockRepository(), 4)
	svr := inventoryservice.NewServer(NewInventoryServiceImpl(inventoryService), server.WithServiceAddr(addr))

	if err := svr.Run(); err != nil {
		log.Println(err.Error())
	}
}
