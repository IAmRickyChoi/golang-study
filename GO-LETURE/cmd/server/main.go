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

	compRepo := repository.NewCompRepository(repository.DB)
	authService := service.NewAuthService(compRepo)

	r := gin.Default()
	authMiddleware := authService.AuthMiddleware()
	authorized := r.Group("/", authMiddleware)

	authorized.POST("/board/add", boardService.RegisterBoard)
	authorized.GET("/board/get/:id", boardService.GetBoardById)
	authorized.PUT("/board/update", boardService.UpdateBoard)
	authorized.DELETE("/board/delete/:id", boardService.DeleteBoard)
	r.POST("/auth/token", authService.MakeToken)

	r.Run(":9900")
}
