package repository

import (
	"go-api/internal/model"

	"gorm.io/gorm"
)

type CompRepository struct {
	db *gorm.DB
}

func NewCompRepository(db *gorm.DB) *CompRepository {
	return &CompRepository{db: db}
}

func (r *CompRepository) Update(comp *model.Comp) error {
	return r.db.Save(comp).Error
}

func (r *CompRepository) FindById(id string) (*model.Comp, error) {
	var comp model.Comp
	err := r.db.First(&comp, "id = ? ", id).Error
	return &comp, err
}

func (r *CompRepository) FindByIdAndToken(id string, token string) (*model.Comp, error) {
	var comp model.Comp
	err := r.db.First(&comp, "id = ?  AND token = ?", id, token).Error
	return &comp, err
}
