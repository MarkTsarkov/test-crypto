package model

import "time"

type Banner struct {
	ID   int    `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
}

type Click struct {
	BannerID int
	Ts       time.Time
}

type ClickStat struct {
	BannerID int       `db:"banner_id" json:"banner_id"`
	Ts       time.Time `db:"ts" json:"ts"`
	Count    int       `db:"count" json:"count"`
}
