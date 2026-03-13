// Package service 签名服务
package service

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"gorm.io/gorm"

	"license-server/internal/model"
)

// cryptoSHA256 别名，用于 SignPKCS1v15
var cryptoSHA256 = crypto.SHA256

// SigningService 签名服务
type SigningService struct {
	db *gorm.DB
}

// NewSigningService 创建签名服务
func NewSigningService(db *gorm.DB) *SigningService {
	return &SigningService{db: db}
}

// CreateSigningKeyRequest 创建签名密钥请求
type CreateSigningKeyRequest struct {
	TenantID  uint64 `json:"tenant_id"`
	Name      string `json:"name" binding:"required"`
	Algorithm string `json:"algorithm"` // rsa2048, rsa4096
}

// SigningKeyResponse 签名密钥响应
type SigningKeyResponse struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"public_key"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

// CreateSigningKey 创建签名密钥对
func (s *SigningService) CreateSigningKey(tenantID uint64, name, algorithm string) (*model.SigningKey, error) {
	var privateKey *rsa.PrivateKey
	var err error

	// 根据 algorithm 确定密钥长度
	bits := 2048
	if algorithm == "rsa4096" {
		bits = 4096
	}

	// 生成 RSA 密钥对
	privateKey, err = rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// 编码公钥
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// 编码私钥
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	key := &model.SigningKey{
		TenantID:   tenantID,
		Name:       name,
		Algorithm:  algorithm,
		PublicKey:  string(publicKeyPEM),
		PrivateKey: string(privateKeyPEM),
		IsActive:   true,
	}

	if err := s.db.Create(key).Error; err != nil {
		return nil, fmt.Errorf("failed to save key: %w", err)
	}

	return key, nil
}

// GetSigningKey 获取签名密钥
func (s *SigningService) GetSigningKey(id uint64) (*model.SigningKey, error) {
	var key model.SigningKey
	if err := s.db.First(&key, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("signing key not found")
		}
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	return &key, nil
}

// GetActiveSigningKey 获取活跃的签名密钥
func (s *SigningService) GetActiveSigningKey(tenantID uint64) (*model.SigningKey, error) {
	var key model.SigningKey
	if err := s.db.Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Order("created_at DESC").
		First(&key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no active signing key")
		}
		return nil, fmt.Errorf("failed to get active key: %w", err)
	}
	return &key, nil
}

// ListSigningKeys 列出签名密钥
func (s *SigningService) ListSigningKeys(tenantID uint64) ([]model.SigningKey, error) {
	var keys []model.SigningKey
	if err := s.db.Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	return keys, nil
}

// DeleteSigningKey 删除签名密钥
func (s *SigningService) DeleteSigningKey(id uint64) error {
	if err := s.db.Delete(&model.SigningKey{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	return nil
}

// SetActiveKey 设置活跃密钥
func (s *SigningService) SetActiveKey(tenantID, keyID uint64) error {
	// 先将所有密钥设为非活跃
	if err := s.db.Model(&model.SigningKey{}).
		Where("tenant_id = ?", tenantID).
		Update("is_active", false).Error; err != nil {
		return fmt.Errorf("failed to deactivate keys: %w", err)
	}

	// 设置指定密钥为活跃
	if err := s.db.Model(&model.SigningKey{}).
		Where("id = ?", keyID).
		Update("is_active", true).Error; err != nil {
		return fmt.Errorf("failed to activate key: %w", err)
	}

	return nil
}

// SignData 对数据签名
func (s *SigningService) SignData(keyID uint64, data []byte) (string, error) {
	key, err := s.GetSigningKey(keyID)
	if err != nil {
		return "", err
	}

	// 解析私钥
	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to decode private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// 计算 hash
	hashed := sha256.Sum256(data)

	// 签名
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, cryptoSHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// VerifySignature 验证签名
func (s *SigningService) VerifySignature(publicKeyPEM string, data []byte, signatureBase64 string) error {
	// 解析公钥
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return fmt.Errorf("failed to decode public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	publicKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("not an RSA public key")
	}

	// 解码签名
	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// 计算 hash
	hashed := sha256.Sum256(data)

	// 验证签名
	err = rsa.VerifyPKCS1v15(publicKey, cryptoSHA256, hashed[:], signature)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}