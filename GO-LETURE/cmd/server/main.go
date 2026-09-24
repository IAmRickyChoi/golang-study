package main

import (
	"go-api/internal/config"
	"go-api/internal/repository"
	"go-api/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	repository.InitDB(cfg.DSN())

	boardRepo := repository.NewBoardRepository(repository.DB)
	boardService := service.NewBoardService(boardRepo)

	r := gin.Default()

	r.POST("/board/add", boardService.RegisterBoard)
	r.GET("/board/get/:id", boardService.GetBoardById)

	r.Run(":9900")
}
