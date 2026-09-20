// Package idgen fornece implementações concretas e reutilizáveis de
// geração de identificadores únicos, desacopladas de qualquer regra de
// negócio. Reside em /pkg pois é uma capacidade genérica, sem
// conhecimento do domínio de transferências P2P, potencialmente
// reutilizável por outros serviços.
package idgen

import "github.com/google/uuid"

// UUIDGenerator implementa usecase.IDGenerator usando UUID v4 (aleatório),
// adequado para identificar transações de forma globalmente única sem
// coordenação central.
type UUIDGenerator struct{}

// NewUUIDGenerator constrói um gerador de UUID v4.
func NewUUIDGenerator() *UUIDGenerator {
	return &UUIDGenerator{}
}

// NewID gera e retorna um novo UUID v4 como string.
func (g *UUIDGenerator) NewID() string {
	return uuid.NewString()
}
