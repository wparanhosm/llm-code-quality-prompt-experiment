package domain

import "errors"

// Erros de domínio. São comparados com errors.Is nas camadas superiores
// (usecase traduz, delivery mapeia para status HTTP).
var (
	// ErrInvalidAmount indica valor monetário inválido (negativo ou zero
	// em contextos que exigem valor positivo).
	ErrInvalidAmount = errors.New("valor monetário inválido")

	// ErrInvalidWalletID indica identificador de carteira vazio ou inválido.
	ErrInvalidWalletID = errors.New("identificador de carteira inválido")

	// ErrSameWallet indica tentativa de transferência para a própria carteira.
	ErrSameWallet = errors.New("carteira de origem e destino devem ser diferentes")

	// ErrWalletNotFound indica que a carteira não existe.
	ErrWalletNotFound = errors.New("carteira não encontrada")

	// ErrInsufficientFunds indica saldo insuficiente para a operação.
	ErrInsufficientFunds = errors.New("saldo insuficiente")
)
