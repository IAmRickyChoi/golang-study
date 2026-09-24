package model

import "time"

type Board struct {
	ID        int       `json:"id,omitempty"`
	Title     string    `json:"title,omitempty"`
	Content   string    `json:"content,omitempty"`
	Author    string    `json:"author,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Code      string    `json:"code,omitempty" gorm:"-"`
}

func (Board) TableName() string {
	return "board"
}
