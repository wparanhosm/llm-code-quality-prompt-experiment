// Package domain contém as entidades de negócio puras do sistema de
// transferências P2P. Nenhum arquivo deste pacote pode importar frameworks,
// drivers de banco de dados ou bibliotecas de infraestrutura.
package domain

import (
	"errors"
	"fmt"
)

// Erros de domínio. São sentinelas para permitir que camadas superiores
// (usecase, delivery) façam tratamento específico via errors.Is.
var (
	// ErrInsufficientFunds indica que a carteira de origem não possui saldo
	// suficiente para completar o débito solicitado.
	ErrInsufficientFunds = errors.New("saldo insuficiente para realizar a transferência")

	// ErrInvalidAmount indica que o valor informado para a operação é
	// inválido (zero ou negativo).
	ErrInvalidAmount = errors.New("valor da transferência deve ser positivo")

	// ErrSameWallet indica que a carteira de origem e destino são a mesma,
	// o que não é uma operação de transferência válida.
	ErrSameWallet = errors.New("carteira de origem e destino não podem ser iguais")

	// ErrWalletNotFound indica que a carteira solicitada não existe.
	ErrWalletNotFound = errors.New("carteira não encontrada")

	// ErrWalletInactive indica que a carteira existe mas está inativa e não
	// pode participar de operações financeiras.
	ErrWalletInactive = errors.New("carteira inativa")
)

// Money representa um valor monetário em centavos (menor unidade da moeda).
// Utilizar um inteiro em centavos evita os problemas de precisão de ponto
// flutuante ao lidar com dinheiro.
type Money int64

// NewMoney cria um valor monetário validando que ele é positivo. Operações
// de transferência sempre exigem valores estritamente positivos.
func NewMoney(cents int64) (Money, error) {
	if cents <= 0 {
		return 0, ErrInvalidAmount
	}
	return Money(cents), nil
}

// Int64 retorna o valor em centavos como inteiro nativo, útil para
// persistência e serialização.
func (m Money) Int64() int64 {
	return int64(m)
}

// String formata o valor monetário como "R$ X.YY" para fins de log e
// mensagens de erro legíveis por humanos.
func (m Money) String() string {
	return fmt.Sprintf("R$ %d.%02d", int64(m)/100, int64(m)%100)
}

// WalletStatus representa o estado atual de uma carteira digital.
type WalletStatus string

const (
	// WalletStatusActive indica que a carteira pode realizar operações.
	WalletStatusActive WalletStatus = "ACTIVE"
	// WalletStatusInactive indica que a carteira está bloqueada para
	// operações financeiras (ex: suspensão por fraude, encerramento).
	WalletStatusInactive WalletStatus = "INACTIVE"
)

// Wallet é a entidade que representa uma carteira digital de um usuário,
// com seu saldo atual em centavos.
type Wallet struct {
	ID      string
	OwnerID string
	Balance Money
	Status  WalletStatus
	Version int64 // usado para controle de concorrência otimista em logs/auditoria
}

// NewWallet constrói uma carteira válida garantindo os invariantes mínimos
// da entidade.
func NewWallet(id, ownerID string, balance Money, status WalletStatus) (*Wallet, error) {
	if id == "" {
		return nil, errors.New("id da carteira não pode ser vazio")
	}
	if ownerID == "" {
		return nil, errors.New("owner id da carteira não pode ser vazio")
	}
	if status != WalletStatusActive && status != WalletStatusInactive {
		return nil, fmt.Errorf("status de carteira inválido: %s", status)
	}
	return &Wallet{
		ID:      id,
		OwnerID: ownerID,
		Balance: balance,
		Status:  status,
	}, nil
}

// EnsureActive retorna um erro se a carteira não estiver ativa para operar.
func (w *Wallet) EnsureActive() error {
	if w.Status != WalletStatusActive {
		return fmt.Errorf("%w: carteira %s", ErrWalletInactive, w.ID)
	}
	return nil
}

// Debit remove o valor informado do saldo da carteira. Retorna
// ErrInsufficientFunds caso o saldo seja menor que o valor solicitado.
// A operação é pura: apenas modifica o estado em memória da entidade,
// sem qualquer efeito colateral de persistência.
func (w *Wallet) Debit(amount Money) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if err := w.EnsureActive(); err != nil {
		return err
	}
	if w.Balance < amount {
		return fmt.Errorf("%w: carteira %s possui %s, necessário %s", ErrInsufficientFunds, w.ID, w.Balance, amount)
	}
	w.Balance -= amount
	return nil
}

// Credit adiciona o valor informado ao saldo da carteira.
func (w *Wallet) Credit(amount Money) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if err := w.EnsureActive(); err != nil {
		return err
	}
	w.Balance += amount
	return nil
}
