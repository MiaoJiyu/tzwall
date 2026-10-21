package main

import (
	"log"

	"tzwall/backend/internal/bootstrap"
)

func main() {
	if err := bootstrap.Run(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
