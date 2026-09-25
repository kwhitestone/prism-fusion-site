package model

import "time"

// CasdoorSession records each refresh generation; no bearer credentials are stored.
// It deliberately does not reference the builtin provider's local users table.
type CasdoorSession struct {
	TokenHash       string    `gorm:"primaryKey;size:64"`
	FamilyID        string    `gorm:"index;not null"`
	Subject         string    `gorm:"not null"`
	AccessKey       string    `gorm:"index;not null"`
	AccessExpiresAt time.Time `gorm:"not null"`
	ExpiresAt       time.Time `gorm:"not null"`
	FamilyExpiresAt time.Time `gorm:"index;not null"`
	RotatedAt       *time.Time
	RevokedAt       *time.Time
}

type CasdoorTokenBlacklist struct {
	Key       string    `gorm:"primaryKey;size:64"`
	ExpiresAt time.Time `gorm:"index;not null"`
}
