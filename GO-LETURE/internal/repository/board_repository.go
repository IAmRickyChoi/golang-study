package repository

import (
	"go-api/internal/model"

	"gorm.io/gorm"
)

type BoardRepository struct {
	db *gorm.DB
}

func NewBoardRepository(db *gorm.DB) *BoardRepository {
	return &BoardRepository{db: db}
}

func (r *BoardRepository) Create(board *model.Board) error {
	return r.db.Create(board).Error
}

func (r *BoardRepository) FindById(id int) (*model.Board, error) {
	var board model.Board
	err := r.db.First(&board, id).Error
	return &board, err
}

func (r *BoardRepository) Update(board *model.Board) error {
	return r.db.Save(board).Error
}

func (r *BoardRepository) Delete(id int) error {
	return r.db.Delete(&model.Board{}, id).Error
}
