package service

import (
	"go-api/internal/model"
	"go-api/internal/repository"

	"github.com/gin-gonic/gin"
)

type BoardService struct {
	repo *repository.BoardRepository
}

func NewBoardService(repo *repository.BoardRepository) *BoardService {
	return &BoardService{repo: repo}
}

func (s *BoardService) RegisterBoard(c *gin.Context) {
	var board model.Board

	if err := c.ShouldBindJSON(&board); err != nil {
		c.JSON(400, gin.H{"code": "400", "message": err.Error()})
		return
	}

	if err := s.repo.Create(&board); err != nil {
		c.JSON(500, gin.H{"code": "500", "message": err.Error()})
		return
	}

	board.Code = "200"
	c.JSON(200, board)
}

func (s *BoardService) GetBoardById(c *gin.Context) {
	var req struct {
		Id int `uri:"id" binding:"required"`
	}

	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(400, gin.H{"code": "400", "message": err.Error()})
		return
	}

	board, err := s.repo.FindById(req.Id)
	if err != nil {
		c.JSON(500, gin.H{"code": "500", "message": err.Error()})
		return
	}

	board.Code = "200"
	c.JSON(200, board)
}
