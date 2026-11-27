package tables

import (
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/framework/encrypt"
	"gorm.io/gorm"
)

// TableGuardrailProvider is one guardrail profile: the provider it runs (secrets, regex or
// presidio), the policy name shown in intervention messages, and the provider config JSON
// (snake_case keys, see the guardrails plugin). IDs are explicit — the config.json sync seeds
// them — so the column is not autoincrement; an insert with no ID takes max(id)+1.
type TableGuardrailProvider struct {
	ID           uint      `gorm:"primaryKey;autoIncrement:false" json:"id"`
	ProviderName string    `gorm:"type:varchar(64);index;not null" json:"provider_name"`
	PolicyName   string    `gorm:"type:varchar(255);not null" json:"policy_name"`
	Enabled      bool      `gorm:"not null" json:"enabled"`
	Timeout      int       `gorm:"not null" json:"timeout"` // seconds, 0 => provider default
	ConfigJSON   string    `gorm:"type:text" json:"-"`      // plaintext in memory, encrypted at rest
	ConfigHash   string    `gorm:"type:varchar(255)" json:"config_hash"`
	CreatedAt    time.Time `gorm:"index;not null" json:"created_at"`
	UpdatedAt    time.Time `gorm:"index;not null" json:"updated_at"`

	// EncryptionStatus records whether ConfigJSON is ciphertext, so rows written before
	// encryption was enabled remain readable.
	EncryptionStatus string `gorm:"type:varchar(20);default:'plain_text'" json:"-"`
}

// TableName pins the physical table name. It has no enterprise_ prefix so a future
// coexisting enterprise build can share it.
func (TableGuardrailProvider) TableName() string { return "guardrail_providers" }

// BeforeSave encrypts ConfigJSON when encryption is enabled.
func (p *TableGuardrailProvider) BeforeSave(*gorm.DB) error {
	if !encrypt.IsEnabled() || p.ConfigJSON == "" {
		p.EncryptionStatus = EncryptionStatusPlainText
		return nil
	}
	encrypted, err := encrypt.Encrypt(p.ConfigJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt guardrail provider config: %w", err)
	}
	p.ConfigJSON = encrypted
	p.EncryptionStatus = EncryptionStatusEncrypted
	return nil
}

// AfterFind decrypts ConfigJSON for rows marked encrypted.
func (p *TableGuardrailProvider) AfterFind(*gorm.DB) error {
	if p.EncryptionStatus != EncryptionStatusEncrypted || p.ConfigJSON == "" {
		return nil
	}
	decrypted, err := encrypt.Decrypt(p.ConfigJSON)
	if err != nil {
		return fmt.Errorf("failed to decrypt guardrail provider config: %w", err)
	}
	p.ConfigJSON = decrypted
	return nil
}

// TableGuardrailRule is one guardrail rule: a CEL expression selecting requests, the profiles
// it runs in order, and its sampling/timeout settings.
type TableGuardrailRule struct {
	ID            uint      `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Name          string    `gorm:"type:varchar(255);not null" json:"name"`
	Description   string    `gorm:"type:text" json:"description"`
	Enabled       bool      `gorm:"not null" json:"enabled"`
	Target        string    `gorm:"type:varchar(32)" json:"target"`   // "llm"
	ApplyTo       string    `gorm:"type:varchar(32)" json:"apply_to"` // "input"
	CELExpression string    `gorm:"type:text;not null" json:"cel_expression"`
	QueryJSON     string    `gorm:"type:text" json:"-"`
	SamplingRate  *int      `gorm:"type:int" json:"sampling_rate,omitempty"` // nil => 100
	Timeout       int       `gorm:"not null" json:"timeout"`                 // seconds, 0 => rule default
	ConfigHash    string    `gorm:"type:varchar(255)" json:"config_hash"`
	CreatedAt     time.Time `gorm:"index;not null" json:"created_at"`
	UpdatedAt     time.Time `gorm:"index;not null" json:"updated_at"`

	// ProviderConfigIDsJSON is the stored, ordered list of guardrail_providers IDs.
	ProviderConfigIDsJSON string `gorm:"type:text" json:"-"`
	// ProviderConfigIDs is the decoded list in execution order (not stored directly).
	ProviderConfigIDs []uint `gorm:"-" json:"provider_config_ids,omitempty"`
}

// TableName pins the physical table name.
func (TableGuardrailRule) TableName() string { return "guardrail_rules" }

// BeforeSave serializes ProviderConfigIDs, preserving order, when the caller set the decoded
// slice rather than the raw JSON.
func (r *TableGuardrailRule) BeforeSave(*gorm.DB) error {
	if r.ProviderConfigIDs == nil {
		return nil
	}
	data, err := sonic.Marshal(r.ProviderConfigIDs)
	if err != nil {
		return err
	}
	r.ProviderConfigIDsJSON = string(data)
	return nil
}

// AfterFind decodes ProviderConfigIDs from the stored JSON.
func (r *TableGuardrailRule) AfterFind(*gorm.DB) error {
	if strings.TrimSpace(r.ProviderConfigIDsJSON) == "" {
		r.ProviderConfigIDs = nil
		return nil
	}
	return sonic.Unmarshal([]byte(r.ProviderConfigIDsJSON), &r.ProviderConfigIDs)
}
