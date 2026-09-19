package tables

import "time"

const ConfigRevisionSingletonID uint = 1

// TableConfigRevision stores the global runtime-configuration revision.
// The table contains exactly one row identified by ConfigRevisionSingletonID.
type TableConfigRevision struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Revision  int64     `gorm:"not null" json:"revision"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

// TableName sets the table name.
func (TableConfigRevision) TableName() string { return "config_revisions" }
