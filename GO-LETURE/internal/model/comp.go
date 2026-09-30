package model

type Comp struct {
	ID    string `json:"id,omitempty" gorm:"primaryKey"`
	Name  string `json:"name,omitempty"`
	Token string `json:"token,omitempty"`
	Code  string `json:"code,omitempty" gorm:"-"`
}

func (c *Comp) TableName() string {
	return "comp"
}
