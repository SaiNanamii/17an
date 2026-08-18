package models

import "time"

// Field names match the actual imported schema (inspected via \d on the
// loaded dump on 2026-08-18).

type User struct {
	UserID                uint64     `gorm:"column:user_id;primaryKey" json:"user_id"`
	UserName              string     `gorm:"column:user_name" json:"user_name"`
	UserEmail             *string    `gorm:"column:user_email" json:"user_email"`
	UserPwd               string     `gorm:"column:user_pwd" json:"-"`
	Status                int16      `gorm:"column:status" json:"status"`
	FullName              string     `gorm:"column:full_name" json:"full_name"`
	Sex                   int16      `gorm:"column:sex" json:"sex"`
	BirthDate             *time.Time `gorm:"column:birth_date" json:"birth_date"`
	Location              string     `gorm:"column:location" json:"location"`
	Msisdn                *string    `gorm:"column:msisdn" json:"msisdn"`
	Messenger             string     `gorm:"column:messenger" json:"messenger"`
	FlagEmail             int16      `gorm:"column:flag_email" json:"flag_email"`
	FlagMessenger         int16      `gorm:"column:flag_messenger" json:"flag_messenger"`
	FlagBirthdate         int16      `gorm:"column:flag_birthdate" json:"flag_birthdate"`
	FlagHp                int16      `gorm:"column:flag_hp" json:"flag_hp"`
	FlagImg               int16      `gorm:"column:flag_img" json:"flag_img"`
	Occupation            string     `gorm:"column:occupation" json:"occupation"`
	Company               string     `gorm:"column:company" json:"company"`
	Schools               string     `gorm:"column:schools" json:"schools"`
	Hobbies               string     `gorm:"column:hobbies" json:"hobbies"`
	Relationship          int16      `gorm:"column:relationship" json:"relationship"`
	ActivationCode        string     `gorm:"column:activation_code" json:"-"`
	AboutMe               string     `gorm:"column:about_me" json:"about_me"`
	LastLogin             *time.Time `gorm:"column:last_login" json:"last_login"`
	Deposit               float64    `gorm:"column:deposit" json:"deposit"`
	CreateBy              int        `gorm:"column:create_by" json:"create_by"`
	CreateTime            *time.Time `gorm:"column:create_time" json:"create_time"`
	UpdateBy              int        `gorm:"column:update_by" json:"update_by"`
	UpdateTime            *time.Time `gorm:"column:update_time" json:"update_time"`
	EmailNew              string     `gorm:"column:email_new" json:"email_new"`
	EmailCancelCode       string     `gorm:"column:email_cancel_code" json:"-"`
	EmailConfirmCode      string     `gorm:"column:email_confirm_code" json:"-"`
	ResetPasswordCode     string     `gorm:"column:reset_password_code" json:"-"`
	ProfileEffectiveDate  *time.Time `gorm:"column:profile_effective_date" json:"profile_effective_date"`
	StatusData            int16      `gorm:"column:status_data" json:"status_data"`
	ShopInfo              string     `gorm:"column:shop_info" json:"shop_info"`
	Lang                  string     `gorm:"column:lang" json:"lang"`
	UserPwd1              string     `gorm:"column:user_pwd_1" json:"-"`
	UniqChar              string     `gorm:"column:uniq_char" json:"uniq_char"`
}

func (User) TableName() string { return "ws_user" }

type Order struct {
	OrderID     uint64    `gorm:"column:order_id;primaryKey" json:"order_id"`
	UserID      uint64    `gorm:"column:user_id" json:"user_id"`
	OrderDate   time.Time `gorm:"column:order_date" json:"order_date"`
	OrderAmount float64   `gorm:"column:order_amount" json:"order_amount"`
	OrderStatus int16     `gorm:"column:order_status" json:"order_status"`
}

func (Order) TableName() string { return "ws_orders" }

type Transaction struct {
	TransactionID     uint64    `gorm:"column:transaction_id;primaryKey" json:"transaction_id"`
	OrderID           uint64    `gorm:"column:order_id" json:"order_id"`
	TransactionDate   time.Time `gorm:"column:transaction_date" json:"transaction_date"`
	TransactionAmount float64   `gorm:"column:transaction_amount" json:"transaction_amount"`
	TransactionType   string    `gorm:"column:transaction_type" json:"transaction_type"`
	Status            string    `gorm:"column:status" json:"status"`
}

func (Transaction) TableName() string { return "ws_transactions" }

type UserActivity struct {
	ActivityID        uint64    `gorm:"column:activity_id;primaryKey" json:"activity_id"`
	UserID            uint64    `gorm:"column:user_id" json:"user_id"`
	ActivityType      string    `gorm:"column:activity_type" json:"activity_type"`
	ActivityTimestamp time.Time `gorm:"column:activity_timestamp" json:"activity_timestamp"`
	IPAddress         string    `gorm:"column:ip_address" json:"ip_address"`
}

func (UserActivity) TableName() string { return "ws_user_activity" }

type UserPreference struct {
	PreferenceID uint64 `gorm:"column:preference_id;primaryKey" json:"preference_id"`
	UserID       uint64 `gorm:"column:user_id" json:"user_id"`
	Theme        string `gorm:"column:theme" json:"theme"`
	Language     string `gorm:"column:language" json:"language"`
}

func (UserPreference) TableName() string { return "ws_user_preferences" }
